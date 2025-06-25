package models

import (
	"time"
)

type AtributoTercero struct {
	IDAtributo      int       `json:"id_atributo"`
	IDUsuario       int       `json:"id_usuario"`
	Atributos       []string  `json:"atributos"` // JSONB en la base de datos
	FechaAsignacion time.Time `json:"fecha_asignacion"`
	AsignadoPor     int       `json:"asignado_por"` // id del administrador o autoridad que asigna
}
