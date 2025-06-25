// backend/models/monitoreo.go
package models

import "time"

// EventoMonitoreo representa una fila de la tabla auditoria_eventos
type EventoMonitoreo struct {
	IDEvento      int       `json:"id_evento"`      // PK de auditoria_eventos
	IDUsuario     int       `json:"id_usuario"`     // Usuario que disparó la acción
	Accion        string    `json:"accion"`         // "INSERT", "UPDATE", "DELETE", etc.
	TablaAfectada string    `json:"tabla_afectada"` // Nombre de la tabla afectada (p.ej. "consentimientos")
	RegistroID    int       `json:"registro_id"`    // El ID (PK) de la fila afectada en esa tabla
	Descripcion   string    `json:"descripcion"`    // Texto libre con detalles de la operación
	FechaEvento   time.Time `json:"fecha_evento"`   // Marca de tiempo en que ocurrió el evento
}
