package models

import "time"

type PoliticaPrivacidad struct {
	ID          int       `json:"id_politica"`
	Titulo      string    `json:"titulo"`
	Descripcion string    `json:"descripcion"`
	FechaInicio time.Time `json:"fecha_inicio"`
	FechaFin    time.Time `json:"fecha_fin"`
}
