package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"

	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/db"
)

func main() {
	// Works whether you run from the repo root or from backend/.
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "up":
		if err := db.MigrateUp(dsn); err != nil {
			log.Fatalf("migrate up: %v", err)
		}
		fmt.Println("migrations applied")
	case "down":
		n := 1
		if len(os.Args) > 2 {
			v, err := strconv.Atoi(os.Args[2])
			if err != nil || v < 1 {
				log.Fatal("down takes a positive number of steps")
			}
			n = v
		}
		if err := db.MigrateSteps(dsn, -n); err != nil {
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Printf("rolled back %d step(s)\n", n)
	case "version":
		v, dirty, err := db.MigrateVersion(dsn)
		if err != nil {
			log.Fatalf("migrate version: %v", err)
		}
		fmt.Printf("version: %d, dirty: %v\n", v, dirty)
	default:
		usage()
	}
}

func usage() {
	fmt.Println("usage: go run ./cmd/migrate <up | down [steps] | version>")
	os.Exit(2)
}
