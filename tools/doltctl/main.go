package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"go-backend/internal/backend/config"
)

func main() {
	_ = godotenv.Load()
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	envCfg, err := config.ParseEnv(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("can't parse environment")
	}

	log.Debug().Any("env", envCfg).Msg("loaded env")

	dbCfg := mysql.Config{
		User:                 envCfg.Database.User,
		Passwd:               envCfg.Database.Password,
		Net:                  envCfg.Database.Net,
		Addr:                 envCfg.Database.Host,
		DBName:               envCfg.Database.Name,
		AllowNativePasswords: true,
	}
	db, err := sql.Open("mysql", dbCfg.FormatDSN())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to database")
	}
	go func() {
		<-ctx.Done()
		log.Info().Msg("interrupt signal")
		os.Exit(0)
	}()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("$: ")
		var query string

		line, tooLong, err := reader.ReadLine()
		if tooLong {
			log.Error().Msg("be shorter")
		}

		query = string(line)

		fmt.Println("read:", query)
		if err != nil {
			log.Err(err).Msg("input")
			continue
		}
		if query == "exit" {
			os.Exit(0)
		}

		res, err := db.QueryContext(ctx, query)
		if err != nil {
			log.Err(err).Msg("query finished")
			continue
		}
		columns, err := res.Columns()
		if err != nil {
			log.Err(err).Msg("scanning column names")
			continue
		}

		fmt.Println(columns)
		for res.Next() {
			kek := make([]any, len(columns))
			for i := range kek {
				kek[i] = lo.ToPtr("")
			}
			if err := res.Scan(kek...); err != nil {
				log.Err(err).Msg("reading row")
			}

			for _, lol := range kek {
				fmt.Printf("%s ", *lol.(*string))
			}
			fmt.Println()
		}
	}
}
