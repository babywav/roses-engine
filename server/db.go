package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DBPool é o pool de conexões global com o PostgreSQL.
var DBPool *pgxpool.Pool

// initDB inicializa o pool de conexões utilizando a DATABASE_URL.
func initDB() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Println("[Database] Alerta: DATABASE_URL não definida no ambiente.")
		return
	}

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Printf("[Database] Erro ao analisar DATABASE_URL: %v", err)
		return
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Printf("[Database] Erro ao criar pool de conexões: %v", err)
		return
	}

	// Testar conexão de forma segura com timeout de 5 segundos.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := pool.Ping(ctx); err != nil {
		log.Printf("[Database] Aviso: Ping ao banco de dados falhou (DATABASE_URL pode estar incompleta ou incorreta): %v", err)
	} else {
		log.Println("[Database] Conexão com o banco de dados estabelecida com sucesso.")
	}

	DBPool = pool
}
