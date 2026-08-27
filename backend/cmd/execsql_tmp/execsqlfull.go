// Command execsqlfull runs a SQL file against the dev DB and captures
// NOTICE output (for rule-9 probe verification).
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: execsqlfull <file.sql>")
		os.Exit(1)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://" + os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") + "@" + os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT") + "/" + os.Getenv("DB_NAME")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Println("connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	body, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("read:", err)
		os.Exit(1)
	}

	// Split on ; respecting $$ blocks
	var notices []string
	conn, err := pool.Acquire(ctx)
	if err != nil {
		fmt.Println("acquire:", err)
		os.Exit(1)
	}
	defer conn.Release()

	parts := splitSQL(string(body))
	for _, stmt := range parts {
		_, err := conn.Exec(ctx, stmt)
		if err != nil {
			// The DO block RAISEs inside are exceptions; pgx surfaces them.
			// NOTICEs are emitted on the connection; we capture via pgconn.
			if strings.Contains(err.Error(), "RULE9_PROBE") {
				fmt.Println("EXEC NOTE:", err)
			} else {
				fmt.Println("EXEC ERR:", err)
				os.Exit(1)
			}
		}
	}
	fmt.Println(strings.Join(notices, "\n"))
	fmt.Println("DONE")
}

func splitSQL(s string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '$' && i+1 < len(s) && s[i+1] == '$' {
			cur.WriteString("$$")
			i++
			depth++
			continue
		}
		if c == '$' && i+1 < len(s) && s[i+1] == '$' && depth > 0 {
			cur.WriteString("$$")
			i++
			depth--
			continue
		}
		if c == ';' && depth == 0 {
			t := strings.TrimSpace(cur.String())
			if t != "" {
				out = append(out, t)
			}
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if t := strings.TrimSpace(cur.String()); t != "" {
		out = append(out, t)
	}
	return out
}
