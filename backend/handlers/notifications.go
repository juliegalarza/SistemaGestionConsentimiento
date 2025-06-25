// handlers/notifications.go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"backend/db"
	"backend/models"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgconn"
)

// GetNotificaciones maneja GET /api/notificaciones
// Devuelve todas las notificaciones de un usuario, ordenadas por fecha de creación.
func GetNotificaciones(w http.ResponseWriter, r *http.Request) {
	userIDVal := r.Context().Value(CtxUserIDKey)
	userID, ok := userIDVal.(int)
	if !ok {
		http.Error(w, "Usuario no autenticado", http.StatusUnauthorized)
		return
	}

	rows, err := db.Pool.Query(r.Context(), `
        SELECT id_notificacion, id_usuario, tipo, referencia_tabla, referencia_id,
               mensaje, url_recurso, enviado_email, leido, fecha_creacion, fecha_leido
        FROM notificaciones
        WHERE id_usuario = $1
        ORDER BY fecha_creacion DESC
    `, userID)
	if err != nil {
		http.Error(w, "Error al leer notificaciones", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var notifs []models.Notificacion
	for rows.Next() {
		var n models.Notificacion
		if err := rows.Scan(
			&n.ID,
			&n.UsuarioID,
			&n.Tipo,
			&n.ReferenciaTabla,
			&n.ReferenciaID,
			&n.Mensaje,
			&n.URLRecurso,
			&n.EnviadoEmail,
			&n.Leido,
			&n.FechaCreacion,
			&n.FechaLeido,
		); err != nil {
			http.Error(w, "Error procesando notificaciones", http.StatusInternalServerError)
			return
		}
		notifs = append(notifs, n)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifs)
}

// GetUnreadCount maneja GET /api/notificaciones/count
// Devuelve el número de notificaciones no leídas para mostrar un badge en la UI.
func GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userIDVal := r.Context().Value(CtxUserIDKey)
	userID, ok := userIDVal.(int)
	if !ok {
		http.Error(w, "Usuario no autenticado", http.StatusUnauthorized)
		return
	}

	var count int
	err := db.Pool.QueryRow(r.Context(), `
        SELECT COUNT(*)
        FROM notificaciones
        WHERE id_usuario = $1 AND leido = FALSE
    `, userID).Scan(&count)
	if err != nil {
		http.Error(w, "Error al contar notificaciones", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"count": count})
}

// MarkAsRead maneja PUT /api/notificaciones/{id}/leer
// Marca una notificación como leída y registra la fecha de lectura.
func MarkAsRead(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID de notificación inválido", http.StatusBadRequest)
		return
	}

	_, err = db.Pool.Exec(r.Context(), `
        UPDATE notificaciones
        SET leido = TRUE, fecha_leido = $2
        WHERE id_notificacion = $1
    `, id, time.Now().UTC())
	if err != nil {
		http.Error(w, "Error al marcar como leída", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CrearNotificacion inserta un nuevo registro en la tabla `notificaciones`.
// Recibe el contexto de la petición para mantener trazabilidad y cancelación.
func CrearNotificacion(ctx context.Context, n *models.Notificacion) error {
	const query = `
        INSERT INTO notificaciones
          (id_usuario, tipo, referencia_tabla, referencia_id, mensaje, url_recurso,
           enviado_email, leido, fecha_creacion)
        VALUES
          ($1, $2, $3, $4, $5, $6, FALSE, FALSE, $7)
        RETURNING id_notificacion
    `
	return db.Pool.QueryRow(ctx, query,
		n.UsuarioID,
		n.Tipo,
		n.ReferenciaTabla,
		n.ReferenciaID,
		n.Mensaje,
		n.URLRecurso,
		time.Now().UTC(),
	).Scan(&n.ID)
}

func CrearSolicitudAtributoP(w http.ResponseWriter, r *http.Request) {
	var solicitud SolicitudAtributo
	if err := json.NewDecoder(r.Body).Decode(&solicitud); err != nil {
		http.Error(w, "Error al decodificar solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	if solicitud.IDProcesador == 0 || solicitud.IDPolitica == 0 || solicitud.Descripcion == "" {
		http.Error(w, "Faltan campos requeridos", http.StatusBadRequest)
		return
	}

	const insertSQL = `
		INSERT INTO solicitudes_atributo (
			id_procesador, id_politica, atributo, descripcion, fecha_creacion, estado
		) VALUES ($1, $2, $3, $4, $5, 'pendiente')
		RETURNING id_solicitud;
	`

	var idSolicitud int
	err := db.Pool.QueryRow(context.Background(), insertSQL,
		solicitud.IDProcesador,
		solicitud.IDPolitica,
		solicitud.Atributo,
		solicitud.Descripcion,
		time.Now(),
	).Scan(&idSolicitud)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			http.Error(w, "Error de PostgreSQL: "+pgErr.Message, http.StatusInternalServerError)
			return
		}
		http.Error(w, "Error al guardar la solicitud: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 🔔 Enviar notificación a todos los controladores (rol = 2)
	const queryControladores = `
		SELECT id_usuario FROM usuarios_roles WHERE id_rol = 2
	`
	rows, err := db.Pool.Query(r.Context(), queryControladores)
	if err != nil {
		http.Error(w, "Error buscando controladores", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var idControlador int
		if err := rows.Scan(&idControlador); err != nil {
			continue
		}

		notificacion := &models.Notificacion{
			UsuarioID:       idControlador,
			Tipo:            "solicitud_atributo",
			ReferenciaTabla: "solicitudes_atributo",
			ReferenciaID:    idSolicitud,
			Mensaje:         "Tienes una nueva solicitud de atributo/dato por parte de un procesador.",
			URLRecurso:      ptrString("/controlador/solicitudes"),
		}

		_ = CrearNotificacion(r.Context(), notificacion) // ignorar error individual
	}

	// ✅ Respuesta final
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensaje":      "Solicitud enviada correctamente",
		"id_solicitud": idSolicitud,
		"estado":       "pendiente",
	})
}

func ptrString(s string) *string {
	return &s
}
