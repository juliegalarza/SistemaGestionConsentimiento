// backend/models/notificacion.go
package models

import "time"

// Notificacion representa un aviso generado por el sistema.
// Corresponde a la tabla "notificaciones" en la base de datos.
type Notificacion struct {
	ID              int        `json:"id_notificacion" db:"id_notificacion"`
	UsuarioID       int        `json:"id_usuario"       db:"id_usuario"`
	Tipo            string     `json:"tipo"             db:"tipo"`
	ReferenciaTabla string     `json:"referencia_tabla" db:"referencia_tabla"`
	ReferenciaID    int        `json:"referencia_id"    db:"referencia_id"`
	Mensaje         string     `json:"mensaje"          db:"mensaje"`
	URLRecurso      *string    `json:"url_recurso,omitempty" db:"url_recurso"`
	EnviadoEmail    bool       `json:"enviado_email"    db:"enviado_email"`
	Leido           bool       `json:"leido"            db:"leido"`
	FechaCreacion   time.Time  `json:"fecha_creacion"   db:"fecha_creacion"`
	FechaLeido      *time.Time `json:"fecha_leido,omitempty" db:"fecha_leido"`
}
