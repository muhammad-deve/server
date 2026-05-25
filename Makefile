PORT ?= 8080

run:
	go run . http $(PORT)
