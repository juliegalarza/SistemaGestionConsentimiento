package models

import "time"

// SolicitudAtributo representa una petición de un procesador para agregar
// un nuevo atributo a una política
type SolicitudAtributo struct {
	IDSolicitud   int       `json:"id_solicitud"`
	IDSolicitante int       `json:"id_solicitante"`
	IDPolitica    int       `json:"id_politica"`
	Atributo      string    `json:"atributo"`
	Descripcion   string    `json:"descripcion"`
	FechaCreacion time.Time `json:"fecha_creacion"`
}
