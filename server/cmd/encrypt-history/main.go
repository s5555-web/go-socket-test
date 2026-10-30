// Usage: encrypt-history [-apply] /etc/signal-web/config.yaml
package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"sket/internal/api"
	"sket/internal/config"
)

func main() {
	apply := flag.Bool("apply", false, "commit encryption migration")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "config path required")
		os.Exit(1)
	}
	cfg, err := config.Load(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration unavailable")
		os.Exit(1)
	}
	db, err := sql.Open("mysql", cfg.Database.DSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "database unavailable")
		os.Exit(1)
	}
	defer db.Close()
	count, err := api.EncryptLegacySupport(db, *apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("legacy support messages: %d; committed: %t\n", count, *apply)
}
