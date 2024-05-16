package main

import (
	"golang.org/x/sys/unix"
	"log"
)

func Init() {
	err := unix.Pledge("stdio rpath inet dns", "")
	if err != nil {
		log.Fatalf("Failed to initialize pledge: %v\n", err)
	}
}
