package db

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool principal (consentimientos)
var Pool *pgxpool.Pool

// PoolDatos para la BD de datos_personales
var ConnDatos *pgxpool.Pool

// ConectarDB inicializa el pool contra la BD de consentimientos
func ConectarDB() {
	var err error
	Pool, err = pgxpool.New(context.Background(),
		"postgres://postgres:postgres@localhost:5432/consentimientos?search_path=public",
	)
	if err != nil {
		log.Fatal("Error creando pool de consentimientos:", err)
	}

	// Comprobación rápida
	var exists bool
	err = Pool.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM usuarios_roles)",
	).Scan(&exists)
	if err != nil {
		log.Fatal("Error comprobando usuarios_roles:", err)
	}
	fmt.Println("¿usuarios_roles existe?:", exists)
}

// ConectarDatosPersonales inicializa el pool contra la BD de datos_personales
func ConectarDatosPersonales() {
	var err error
	ConnDatos, err = pgxpool.New(context.Background(),
		"postgres://postgres:postgres@localhost:5432/datos_personales",
	)
	if err != nil {
		log.Fatal("Error creando pool de datos_personales:", err)
	}
	fmt.Println("Pool datos_personales conectado")
}
