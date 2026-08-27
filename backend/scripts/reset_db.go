//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// Connect to PostgreSQL default database (postgres) to drop/create database
	connStr := "postgresql://postgres:labuda123@localhost:5432/postgres?sslmode=disable"
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	fmt.Println("========================================")
	fmt.Println("DATABASE RESET")
	fmt.Println("========================================")

	// Drop existing database
	fmt.Println("Step 1: Dropping database 'labuda_db'...")
	_, err = db.Exec("DROP DATABASE IF EXISTS labuda_db")
	if err != nil {
		log.Fatal("Failed to drop database:", err)
	}
	fmt.Println("✓ Database dropped")

	// Create new database
	fmt.Println("Step 2: Creating new database 'labuda_db'...")
	_, err = db.Exec("CREATE DATABASE labuda_db")
	if err != nil {
		log.Fatal("Failed to create database:", err)
	}
	fmt.Println("✓ Database created")

	fmt.Println("========================================")
	fmt.Println("DATABASE RESET COMPLETE")
	fmt.Println("========================================")
}
