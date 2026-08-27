package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	connStr := "host=localhost port=5432 user=labuda password=labuda123 dbname=labuda sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	rows, err := db.Query(`
		SELECT conname
		FROM pg_constraint
		WHERE conname LIKE 'negotiation_messages%'
		ORDER BY conname
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Constraints for negotiation_messages:")
	for rows.Next() {
		var name string
		rows.Scan(&name)
		fmt.Printf("  • %s\n", name)
	}
}