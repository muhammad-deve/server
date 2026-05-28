"use client"

import { useState, useRef } from "react"
import { Check, Copy } from "lucide-react"
import { motion, AnimatePresence } from "framer-motion"

interface CopyButtonProps {
  text: string
  className?: string
}

export function CopyButton({ text, className = "" }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)
  const buttonRef = useRef<HTMLButtonElement>(null)

  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation()
    
    // Try clipboard API with fallback
    try {
      await navigator.clipboard.writeText(text)
    } catch {
      // Fallback for iframe/permissions issues - create a temporary textarea
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    
    setCopied(true)
    
    // Add flash animation
    if (buttonRef.current) {
      buttonRef.current.classList.add("copy-flash")
    }
    
    // Remove flash class after animation completes
    setTimeout(() => {
      if (buttonRef.current) {
        buttonRef.current.classList.remove("copy-flash")
      }
    }, 1000)
    
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <button
      ref={buttonRef}
      onClick={handleCopy}
      className={`p-1.5 rounded-md transition-colors duration-200 hover:bg-[var(--goport-border)] ${className}`}
      title="Copy to clipboard"
      aria-label={copied ? "Copied!" : "Copy to clipboard"}
    >
      <AnimatePresence mode="wait" initial={false}>
        <motion.span
          key={copied ? "check" : "copy"}
          initial={{ scale: 0, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          exit={{ scale: 0, opacity: 0 }}
          transition={{ duration: 0.15, ease: "easeOut" }}
          className="flex items-center justify-center"
        >
          {copied ? (
            <Check className="w-4 h-4 text-[var(--goport-success)]" />
          ) : (
            <Copy className="w-4 h-4 text-[var(--goport-text-muted)] hover:text-[var(--goport-text-secondary)]" />
          )}
        </motion.span>
      </AnimatePresence>
    </button>
  )
}
