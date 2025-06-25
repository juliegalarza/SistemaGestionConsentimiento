package models

import "time"

type DatosPersonales struct {
	IDDato          int       `json:"id_dato"`
	IDUsuario       int       `json:"id_usuario"`
	Telefono        []byte    `json:"telefono"`
	Celular         []byte    `json:"celular"`
	Direccion       []byte    `json:"direccion"`
	Ciudad          []byte    `json:"ciudad"`
	Provincia       []byte    `json:"provincia"`
	FechaNacimiento []byte    `json:"fecha_nacimiento"` 
	Genero          []byte    `json:"genero"`
	EstadoCivil     []byte    `json:"estado_civil"`
	FechaCreacion   time.Time `json:"fecha_creacion"`
}
