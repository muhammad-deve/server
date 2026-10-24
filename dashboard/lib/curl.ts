/**
 * Builds the inspector's "copy as cURL" command. Header values and bodies come
 * from the public internet, and the developer pastes the result into their own
 * shell, so every value is quoted for the shell it will land in.
 */

export interface CurlParts {
  method: string
  url: string
  headers: [string, string][]
  body?: string
}

/** The inspector is open on Windows, so the command goes to PowerShell (or cmd). */
export function isWindowsClient(): boolean {
  if (typeof navigator === "undefined") return false
  const uaData = (navigator as Navigator & { userAgentData?: { platform?: string } }).userAgentData
  const platform = uaData?.platform || navigator.platform || ""
  return /^win/i.test(platform) || /windows/i.test(navigator.userAgent || "")
}

// C0 controls, DEL and C1 controls. Pasted raw, they can end a bracketed
// paste early or act as terminal / line-editor commands.
const isControl = (c: number) => c < 0x20 || c === 0x7f || (c >= 0x80 && c <= 0x9f)

const SAFE_METHOD = /^[A-Z]+$/

// curl reads a file for a -d value that starts with @.
const dataFlag = (body: string) => (body.startsWith("@") ? "--data-raw" : "-d")

/**
 * POSIX single quotes, also safe in fish, which reads \' and \\ as escapes
 * inside single quotes: ' and backslash runs go outside the quotes, except a
 * lone backslash before an ordinary character, which means the same in every
 * shell. Control characters other than newline come from printf, so they are
 * never pasted raw.
 */
export function posixQuote(s: string): string {
  const chars = Array.from(s)
  let out = "'"
  for (let i = 0; i < chars.length; i++) {
    const ch = chars[i]
    const c = ch.codePointAt(0)!
    if (ch === "\\") {
      let end = i
      while (chars[end] === "\\") end++
      const next = chars[end]
      const plainNext = next !== undefined && next !== "'" && (next === "\n" || !isControl(next.codePointAt(0)!))
      out += end - i === 1 && plainNext ? "\\" : "'" + "\\\\".repeat(end - i) + "'"
      i = end - 1
    } else if (ch === "'") out += "'\\''"
    else if (ch !== "\n" && isControl(c)) {
      const octal = Array.from(new TextEncoder().encode(ch), (b) => "\\" + b.toString(8).padStart(3, "0")).join("")
      out += `'"$(printf '${octal}')"'`
    } else out += ch
  }
  return out + "'"
}

export function posixCommand(parts: CurlParts): string {
  const method = SAFE_METHOD.test(parts.method) ? parts.method : posixQuote(parts.method)
  const lines = [parts.method === "GET" ? `curl ${posixQuote(parts.url)}` : `curl -X ${method} ${posixQuote(parts.url)}`]
  for (const [name, value] of parts.headers) lines.push(`  -H ${posixQuote(`${name}: ${value}`)}`)
  if (parts.body) lines.push(`  ${dataFlag(parts.body)} ${posixQuote(parts.body)}`)
  return lines.join(" \\\n")
}

/** A PowerShell string expression: single-quoted, with control characters as [char]N. */
function psLiteral(s: string): string {
  const parts: string[] = []
  let text = ""
  const flush = () => {
    // PowerShell also ends single-quoted strings at curly single quotes.
    if (text) parts.push(`'${text.replace(/['‘’‚‛]/g, "$&$&")}'`)
    text = ""
  }
  for (const ch of s) {
    const c = ch.codePointAt(0)!
    if (isControl(c)) {
      flush()
      parts.push(`[char]${c}`)
    } else text += ch
  }
  flush()
  if (parts.length === 0) return "''"
  if (parts.length === 1 && parts[0].startsWith("'")) return parts[0]
  return `(''+${parts.join("+")})`
}

/**
 * Windows PowerShell 5.1 passes native arguments without escaping inner double
 * quotes (and mishandles a trailing backslash), so curl.exe would split such a
 * value into extra options. Those values go through a temp file instead.
 */
const unsafeOnCommandLine = (s: string) => s.includes('"') || s.endsWith("\\")

/** Percent-encodes characters that can't sit in a URL argument on Windows. */
function windowsURL(url: string): string {
  let out = ""
  for (const ch of url) {
    const c = ch.codePointAt(0)!
    if (ch === '"' || ch === "\\" || /\s/.test(ch) || isControl(c)) {
      out += Array.from(new TextEncoder().encode(ch), (b) => "%" + b.toString(16).toUpperCase().padStart(2, "0")).join("")
    } else out += ch
  }
  return out
}

/**
 * Windows: a single line for PowerShell. The leading & makes cmd.exe reject
 * the whole line instead of running parts of it.
 */
export function powershellCommand(parts: CurlParts): string {
  const setup: string[] = []
  const files: string[] = []
  const viaFile = (name: string, value: string) => {
    files.push(name)
    setup.push(`$${name} = New-TemporaryFile; [IO.File]::WriteAllText($${name}, ${psLiteral(value)})`)
    return `"@$${name}"`
  }

  const args: string[] = []
  if (parts.method !== "GET") args.push("-X", SAFE_METHOD.test(parts.method) ? parts.method : psLiteral(parts.method))
  args.push(psLiteral(windowsURL(parts.url)))
  parts.headers.forEach(([name, value], i) => {
    const header = `${name}: ${value}`
    args.push("-H", unsafeOnCommandLine(header) ? viaFile(`header${i + 1}`, header) : psLiteral(header))
  })
  if (parts.body) {
    if (unsafeOnCommandLine(parts.body)) args.push("--data-binary", viaFile("body", parts.body))
    else args.push(dataFlag(parts.body), psLiteral(parts.body))
  }

  const curl = `curl.exe ${args.join(" ")}`
  if (files.length === 0) return `& ${curl}`
  return `& { ${setup.join("; ")}; ${curl}; Remove-Item ${files.map((f) => "$" + f).join(", ")} }`
}

export function curlCommand(parts: CurlParts, windows = isWindowsClient()): string {
  return windows ? powershellCommand(parts) : posixCommand(parts)
}
