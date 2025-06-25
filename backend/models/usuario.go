package models

type Usuario struct {
	ID     int    `json:"id_usuario"`
	Nombre string `json:"nombre"`
	Email  string `json:"email"`
}
