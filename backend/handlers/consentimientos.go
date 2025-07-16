// backend/handlers/consentimientos.go
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"backend/db"
	"backend/models"
)

// ConsentimientoInput representa el JSON de entrada para crear o actualizar un consentimiento.
type ConsentimientoInput struct {
	IDUsuario       int        `json:"id_usuario"`
	IDPolitica      int        `json:"id_politica"`
	Estado          string     `json:"estado"`           // "activo" o "no_aceptado"
	FechaExpiracion *time.Time `json:"fecha_expiracion"` // nil si es rechazo
}

// GuardarConsentimiento crea un nuevo consentimiento (o historial si ya hubo uno) y notifica.
// GuardarConsentimiento crea un nuevo consentimiento (o historial si ya hubo uno) y notifica.
func GuardarConsentimiento(w http.ResponseWriter, r *http.Request) {
	var in ConsentimientoInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	now := time.Now()

	// 1) Leer fecha_fin y título de la política
	var finPol time.Time
	var titulo string
	if err := db.Pool.QueryRow(ctx,
		`SELECT fecha_fin, titulo FROM politicas_privacidad WHERE id_politica=$1`,
		in.IDPolitica,
	).Scan(&finPol, &titulo); err != nil {
		http.Error(w, "Política no encontrada", http.StatusNotFound)
		return
	}

	// 2) Si es activación, validar expiración
	if in.Estado == "activo" {
		if in.FechaExpiracion == nil || in.FechaExpiracion.After(finPol) {
			errMsg := fmt.Sprintf("Fecha_expiracion excede fecha_fin para politica=%d", in.IDPolitica)
			db.Pool.Exec(ctx, `
				INSERT INTO auditoria_eventos
				  (id_usuario, accion, tabla_afectada, descripcion, fecha_evento, exito, error_mensaje)
				VALUES ($1, 'FALLO-INSERT', 'consentimientos', $2, NOW(), false, $3)
			`, in.IDUsuario, errMsg, errMsg)

			http.Error(w,
				"La fecha de expiración no puede exceder la vigencia de la política",
				http.StatusBadRequest,
			)
			return
		}
	}

	// 3) ¿Ya existe un consentimiento no expirado?
	var existingID int
	var estExist string
	err := db.Pool.QueryRow(ctx, `
        SELECT id_consentimiento, estado
          FROM consentimientos
         WHERE id_usuario  = $1
           AND id_politica = $2
           AND estado <> 'expirado'
    `, in.IDUsuario, in.IDPolitica).Scan(&existingID, &estExist)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Error interno: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4) Si existe, manejo de "activo" y de rechazo anterior
	if err == nil {
		switch estExist {
		case "activo":
			http.Error(w,
				"Ya tienes un consentimiento vigente. Revísalo o revócalo.",
				http.StatusConflict,
			)
			return
		case "no_aceptado":
			if in.Estado == "no_aceptado" {
				if _, err := db.Pool.Exec(ctx, `
                    UPDATE consentimientos
                       SET estado = 'no_aceptado',
                           fecha_otorgado   = $1,
                           fecha_expiracion = NULL
                     WHERE id_consentimiento = $2
                `, now, existingID); err != nil {
					http.Error(w, "Error actualizando rechazo", http.StatusInternalServerError)
					return
				}
				url := "/titular/politicas"
				notif := &models.Notificacion{
					UsuarioID:       in.IDUsuario,
					Tipo:            "rechazo_consentimiento",
					ReferenciaTabla: "consentimientos",
					ReferenciaID:    existingID,
					Mensaje:         fmt.Sprintf("Has rechazado la política '%s'.", titulo),
					URLRecurso:      &url,
				}
				if err := CrearNotificacion(ctx, notif); err != nil {
					log.Printf("Error notificando rechazo: %v", err)
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"mensaje": "Política rechazada correctamente"})
				return
			}
		}
	}

	// 5) Insertar nuevo consentimiento
	var nuevoID int
	err = db.Pool.QueryRow(ctx, `
        INSERT INTO consentimientos
          (id_usuario, id_politica, fecha_otorgado, fecha_expiracion, estado)
        VALUES ($1,$2,$3,$4,$5)
        RETURNING id_consentimiento
    `, in.IDUsuario, in.IDPolitica, now, in.FechaExpiracion, in.Estado).Scan(&nuevoID)
	if err != nil {
		db.Pool.Exec(ctx, `
			INSERT INTO auditoria_eventos
			  (id_usuario, accion, tabla_afectada, descripcion, fecha_evento, exito, error_mensaje)
			VALUES ($1, 'FALLO-INSERT', 'consentimientos', $2, NOW(), false, $3)
		`, in.IDUsuario, err.Error(), err.Error())

		http.Error(w, "Error guardando consentimiento: "+err.Error(), http.StatusInternalServerError)
		return
	}

	url2 := "/titular/consentimientos"
	notif2 := &models.Notificacion{
		UsuarioID:       in.IDUsuario,
		Tipo:            "nuevo_consentimiento",
		ReferenciaTabla: "consentimientos",
		ReferenciaID:    nuevoID,
		Mensaje:         fmt.Sprintf("Has otorgado un nuevo consentimiento para '%s'.", titulo),
		URLRecurso:      &url2,
	}
	if err := CrearNotificacion(ctx, notif2); err != nil {
		log.Printf("Error notificando nuevo consentimiento: %v", err)
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Consentimiento registrado correctamente"})
}

// RechazarConsentimiento maneja el “No Aceptar” desde la UI.
func RechazarConsentimiento(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDUsuario  int `json:"id_usuario"`
		IDPolitica int `json:"id_politica"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	now := time.Now()

	// 1) Actualizar estado
	res, err := db.Pool.Exec(ctx, `
        UPDATE consentimientos
           SET estado = 'no_aceptado',
               fecha_otorgado   = $1,
               fecha_expiracion = NULL
         WHERE id_usuario  = $2
           AND id_politica = $3
           AND estado      <> 'no_aceptado'
    `, now, in.IDUsuario, in.IDPolitica)
	if err != nil {
		http.Error(w, "Error actualizando rechazo", http.StatusInternalServerError)
		return
	}
	if rows := res.RowsAffected(); rows == 0 {
		http.Error(w, "No hay consentimiento previo para rechazar", http.StatusBadRequest)
		return
	}

	// 2) Obtener título
	var titulo string
	if err := db.Pool.QueryRow(ctx,
		"SELECT titulo FROM politicas_privacidad WHERE id_politica=$1",
		in.IDPolitica,
	).Scan(&titulo); err != nil {
		titulo = "(desconocida)"
	}

	// 3) Notificar al titular
	url := "/titular/politicas"
	msg := fmt.Sprintf("Has rechazado la política '%s'.", titulo)
	notif := &models.Notificacion{
		UsuarioID:       in.IDUsuario,
		Tipo:            "rechazo_consentimiento",
		ReferenciaTabla: "consentimientos",
		ReferenciaID:    in.IDPolitica,
		Mensaje:         msg,
		URLRecurso:      &url,
	}
	if err := CrearNotificacion(ctx, notif); err != nil {
		log.Printf("Error notificando rechazo: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Política rechazada correctamente"})
}

// ObtenerConsentimientosPorUsuario devuelve todos los consentimientos de un usuario,
// incluyendo la bandera revocado_pendiente para el front.
func ObtenerConsentimientosPorUsuario(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query().Get("id_usuario")
	if qs == "" {
		http.Error(w, "Falta id_usuario", http.StatusBadRequest)
		return
	}
	idUsr, err := strconv.Atoi(qs)
	if err != nil {
		http.Error(w, "id_usuario inválido", http.StatusBadRequest)
		return
	}

	rows, err := db.Pool.Query(context.Background(), `
        SELECT DISTINCT ON (c.id_politica)
            c.id_consentimiento,
            c.id_usuario,
            c.id_politica,
            c.fecha_otorgado,
            c.fecha_expiracion,
            c.estado,
            c.revocado_pendiente
          FROM consentimientos c
         WHERE c.id_usuario = $1
         ORDER BY c.id_politica, c.fecha_otorgado DESC
    `, idUsr)
	if err != nil {
		errMsg := fmt.Sprintf("Error consultando consentimientos: %v", err)
		db.Pool.Exec(context.Background(), `
			INSERT INTO auditoria_eventos
			  (id_usuario, accion, tabla_afectada, descripcion, fecha_evento, exito, error_mensaje)
			VALUES ($1, 'FALLO-QUERY', 'consentimientos', $2, NOW(), false, $3)
		`, idUsr, errMsg, errMsg)

		http.Error(w, "Error consultando consentimientos", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lista []models.Consentimiento
	for rows.Next() {
		var c models.Consentimiento
		var fe *time.Time
		if err := rows.Scan(
			&c.IDConsentimiento,
			&c.IDUsuario,
			&c.IDPolitica,
			&c.FechaOtorgado,
			&fe,
			&c.Estado,
			&c.RevocadoPendiente,
		); err != nil {
			http.Error(w, "Error leyendo resultados", http.StatusInternalServerError)
			return
		}
		c.FechaExpiracion = fe
		lista = append(lista, c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}

// ActualizarConsentimiento modifica sólo fecha_expiracion y estado.
func ActualizarConsentimiento(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDConsentimiento int        `json:"id_consentimiento"`
		FechaExpiracion  *time.Time `json:"fecha_expiracion"`
		Estado           string     `json:"estado"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	_, err := db.Pool.Exec(context.Background(), `
        UPDATE consentimientos
           SET fecha_expiracion = $1,
               estado           = $2
         WHERE id_consentimiento = $3
    `, in.FechaExpiracion, in.Estado, in.IDConsentimiento)
	if err != nil {
		http.Error(w, "Error actualizando consentimiento", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Consentimiento actualizado correctamente"})
}

// RevocarConsentimiento marca la revocación como pendiente (24 h), sin cambiar el estado,
// y notifica a titular, controlador y procesadores.
func RevocarConsentimiento(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDUsuario  int `json:"id_usuario"`
		IDPolitica int `json:"id_politica"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	now := time.Now()

	// Marcar sólo la bandera revocado_pendiente
	tag, err := db.Pool.Exec(ctx, `
        UPDATE consentimientos
           SET revocado_pendiente = TRUE,
               fecha_revocacion   = $1
         WHERE id_usuario  = $2
           AND id_politica = $3
           AND estado      = 'activo'
    `, now, in.IDUsuario, in.IDPolitica)
	if err != nil {
		log.Printf("Error marcando revocación pendiente: %v", err)
		http.Error(w, "Error interno al revocar", http.StatusInternalServerError)
		return
	}
	if rows := tag.RowsAffected(); rows == 0 {
		http.Error(w, "No tienes un consentimiento activo para revocar", http.StatusBadRequest)
		return
	}

	// Obtener título de la política
	var titulo string
	if err := db.Pool.QueryRow(ctx,
		"SELECT titulo FROM politicas_privacidad WHERE id_politica=$1",
		in.IDPolitica,
	).Scan(&titulo); err != nil {
		titulo = "(desconocida)"
	}

	// 1) Notificar al TITULAR
	urlTit := "/titular/consentimientos"
	msgTit := fmt.Sprintf("Tu consentimiento para '%s' se revocará en un día.", titulo)
	notifTit := &models.Notificacion{
		UsuarioID:       in.IDUsuario,
		Tipo:            "revocacion_pendiente",
		ReferenciaTabla: "consentimientos",
		ReferenciaID:    in.IDPolitica,
		Mensaje:         msgTit,
		URLRecurso:      &urlTit,
	}
	if err := CrearNotificacion(ctx, notifTit); err != nil {
		log.Printf("Error notificando titular: %v", err)
	}

	// 2) Notificar al CONTROLADOR (rol = 2)
	var idCtrl int
	if err := db.Pool.QueryRow(ctx,
		"SELECT id_usuario FROM usuarios_roles WHERE id_rol=2 LIMIT 1",
	).Scan(&idCtrl); err == nil {
		urlCtl := "/controlador/monitoreo-consentimientos"
		msgCtl := fmt.Sprintf("El titular solicitó revocar el consentimiento para '%s'. Se hará efectivo en un día.", titulo)
		notifCtl := &models.Notificacion{
			UsuarioID:       idCtrl,
			Tipo:            "revocacion_pendiente",
			ReferenciaTabla: "consentimientos",
			ReferenciaID:    in.IDPolitica,
			Mensaje:         msgCtl,
			URLRecurso:      &urlCtl,
		}
		if err := CrearNotificacion(ctx, notifCtl); err != nil {
			log.Printf("Error notificando controlador: %v", err)
		}
	}

	// 3) Notificar a los PROCESADORES que tienen ese atributo
	rows, err := db.Pool.Query(ctx, `
        SELECT id_usuario
          FROM atributos_terceros
         WHERE $1 = ANY(atributos)
    `, titulo)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var idProc int
			if err := rows.Scan(&idProc); err == nil {
				urlPr := "/procesador/consentimientos"
				msgPr := fmt.Sprintf("El titular solicitó revocar el consentimiento para '%s'. Se hará efectivo en un día.", titulo)
				notifPr := &models.Notificacion{
					UsuarioID:       idProc,
					Tipo:            "revocacion_pendiente",
					ReferenciaTabla: "consentimientos",
					ReferenciaID:    in.IDPolitica,
					Mensaje:         msgPr,
					URLRecurso:      &urlPr,
				}
				if err := CrearNotificacion(ctx, notifPr); err != nil {
					log.Printf("Error notificando procesador %d: %v", idProc, err)
				}
			}
		}
	}

	// Responder al cliente
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"mensaje": "Se ha programado la revocación para dentro de 24 horas",
	})
}

// EliminarConsentimiento borra un consentimiento por su ID y notifica.
func EliminarConsentimiento(w http.ResponseWriter, r *http.Request) {
	// 1) Leer parámetro
	q := r.URL.Query().Get("id_consentimiento")
	if q == "" {
		http.Error(w, "Falta id_consentimiento", http.StatusBadRequest)
		return
	}
	cid, err := strconv.Atoi(q)
	if err != nil {
		http.Error(w, "id_consentimiento inválido", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	// 2) Obtener datos previos para notificar
	var idUsr, idPol sql.NullInt64
	var estadoPrevio string
	var fechaExp sql.NullTime
	err = db.Pool.QueryRow(ctx, `
        SELECT id_usuario, id_politica, estado, fecha_expiracion
          FROM consentimientos
         WHERE id_consentimiento=$1
    `, cid).Scan(&idUsr, &idPol, &estadoPrevio, &fechaExp)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Consentimiento no encontrado", http.StatusNotFound)
		} else {
			http.Error(w, "Error consultando consentimiento", http.StatusInternalServerError)
		}
		return
	}

	// 3) Ejecutar DELETE (el trigger maneja auditoría)
	if _, err := db.Pool.Exec(ctx,
		`DELETE FROM consentimientos WHERE id_consentimiento = $1`, cid); err != nil {
		http.Error(w, "Error eliminando consentimiento", http.StatusInternalServerError)
		return
	}

	// 4) Notificar al titular
	if idUsr.Valid {
		url := "/titular/consentimientos"
		msg := fmt.Sprintf(
			"Se ha eliminado tu consentimiento (id=%d) para política %d (estado previo=%s, expiración previa=%s).",
			cid, idPol.Int64, estadoPrevio,
			func() string {
				if fechaExp.Valid {
					return fechaExp.Time.Format("2006-01-02")
				}
				return "N/A"
			}(),
		)
		notif := &models.Notificacion{
			UsuarioID:       int(idUsr.Int64),
			Tipo:            "eliminar_consentimiento",
			ReferenciaTabla: "consentimientos",
			ReferenciaID:    cid,
			Mensaje:         msg,
			URLRecurso:      &url,
		}
		if err := CrearNotificacion(ctx, notif); err != nil {
			log.Printf("Error notificando eliminación al titular: %v", err)
		}
	}

	// 5) Notificar al controlador (rol=2)
	var idCtrl int
	if err := db.Pool.QueryRow(ctx,
		"SELECT id_usuario FROM usuarios_roles WHERE id_rol=2 LIMIT 1",
	).Scan(&idCtrl); err == nil {
		url := "/controlador/monitoreo-consentimientos"
		msg := fmt.Sprintf(
			"Se eliminó el consentimiento id=%d para usuario %d (estado previo=%s).",
			cid, idUsr.Int64, estadoPrevio,
		)
		notif := &models.Notificacion{
			UsuarioID:       idCtrl,
			Tipo:            "eliminar_consentimiento",
			ReferenciaTabla: "consentimientos",
			ReferenciaID:    cid,
			Mensaje:         msg,
			URLRecurso:      &url,
		}
		if err := CrearNotificacion(ctx, notif); err != nil {
			log.Printf("Error notificando eliminación al controlador: %v", err)
		}
	}

	// 6) Responder OK
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Consentimiento eliminado correctamente"})
}
