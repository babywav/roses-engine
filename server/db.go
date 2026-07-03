package main

import (
	"context"
	"log"
	"os"
	"time"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/jackc/pgx/v5"
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

	configStr, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Printf("[Database] Erro ao analisar DATABASE_URL: %v", err)
		return
	}

	// Suporte dinâmico para AWS IAM Database Authentication (RDS Relay/Aurora Serverless v2)
	if strings.Contains(connStr, "rds.amazonaws.com") {
		awsRegion := os.Getenv("AWS_REGION")
		if awsRegion == "" {
			awsRegion = "sa-east-1"
		}
		awsCfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(awsRegion))
		if err == nil {
			configStr.BeforeConnect = func(ctx context.Context, cc *pgx.ConnConfig) error {
				token, err := auth.BuildAuthToken(
					ctx,
					cc.Host+":5432",
					awsCfg.Region,
					cc.User,
					awsCfg.Credentials,
				)
				if err != nil {
					return err
				}
				cc.Password = token
				return nil
			}
		}
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), configStr)
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
