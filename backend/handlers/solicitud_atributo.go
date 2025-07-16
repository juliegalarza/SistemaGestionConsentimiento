package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"backend/db"
	"backend/models"

	"strconv"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgconn"
)

// Estructura adaptada a la tabla real
type SolicitudAtributo struct {
	IDSolicitud      int       `json:"id_solicitud"`
	IDProcesador     int       `json:"id_procesador"`
	IDPolitica       int       `json:"id_politica"`
	Atributo         *string   `json:"atributo,omitempty"`
	Descripcion      string    `json:"descripcion"`
	FechaCreacion    time.Time `json:"fecha_creacion"`
	Estado           string    `json:"estado"`
	DatosSolicitados []string  `json:"datos_solicitados,omitempty"`
	TipoSolicitud    string    `json:"tipo_solicitud"` // 'nuevo' o 'modificacion'
}

// CrearSolicitudAtributo permite a un procesador enviar una solicitud
func CrearSolicitudAtributo(w http.ResponseWriter, r *http.Request) {
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

	// 🔍 Obtener nombre del procesador
	var nombreProcesador string
	err = db.Pool.QueryRow(context.Background(), `
		SELECT nombre FROM usuarios WHERE id_usuario = $1
	`, solicitud.IDProcesador).Scan(&nombreProcesador)
	if err != nil {
		nombreProcesador = "un procesador"
	}

	// 🔔 Notificar a todos los controladores
	rows, err := db.Pool.Query(r.Context(), `SELECT id_usuario FROM usuarios_roles WHERE id_rol = 2`)
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

		// Mensaje con atributo si está presente
		attr := ""
		if solicitud.Atributo != nil {
			attr = " para el atributo '" + *solicitud.Atributo + "'"
		}
		msg := "El procesador '" + nombreProcesador + "' ha solicitado acceso" + attr + "."

		notif := &models.Notificacion{
			UsuarioID:       idControlador,
			Tipo:            "solicitud_atributo",
			ReferenciaTabla: "solicitudes_atributo",
			ReferenciaID:    idSolicitud,
			Mensaje:         msg,
			URLRecurso:      ptrString("/controlador/solicitudes"),
		}
		_ = CrearNotificacion(r.Context(), notif)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensaje":      "Solicitud enviada correctamente",
		"id_solicitud": idSolicitud,
		"estado":       "pendiente",
	})
}

// ObtenerSolicitudesAtributo permite al controlador ver todas las solicitudes de procesadores
func ObtenerSolicitudesAtributo(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(r.Context(), `
		SELECT id_solicitud, id_procesador, id_politica,
		       atributo, descripcion, fecha_creacion,
		       estado, datos_solicitados, tipo_solicitud
		FROM solicitudes_atributo
		ORDER BY fecha_creacion DESC
	`)
	if err != nil {
		http.Error(w, "Error al consultar solicitudes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var solicitudes []SolicitudAtributo
	for rows.Next() {
		var s SolicitudAtributo
		err := rows.Scan(
			&s.IDSolicitud,
			&s.IDProcesador,
			&s.IDPolitica,
			&s.Atributo,
			&s.Descripcion,
			&s.FechaCreacion,
			&s.Estado,
			&s.DatosSolicitados,
			&s.TipoSolicitud,
		)
		if err != nil {
			http.Error(w, "Error procesando resultados", http.StatusInternalServerError)
			return
		}
		solicitudes = append(solicitudes, s)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(solicitudes)
}

// CrearSolicitudModificacion permite al procesador proponer una modificación de política (atributos)
func CrearSolicitudModificacion(w http.ResponseWriter, r *http.Request) {
	var solicitud struct {
		IDProcesador     int      `json:"id_procesador"`
		IDPolitica       int      `json:"id_politica"`
		DatosSolicitados []string `json:"datos_solicitados"`
		Descripcion      string   `json:"descripcion"`
	}

	if err := json.NewDecoder(r.Body).Decode(&solicitud); err != nil {
		http.Error(w, "Error al decodificar la solicitud: "+err.Error(), http.StatusBadRequest)
		return
	}

	if solicitud.IDProcesador == 0 || solicitud.IDPolitica == 0 || len(solicitud.DatosSolicitados) == 0 || solicitud.Descripcion == "" {
		http.Error(w, "Campos obligatorios faltantes", http.StatusBadRequest)
		return
	}

	const insertQuery = `
		INSERT INTO solicitudes_atributo (
			id_procesador, id_politica, datos_solicitados, descripcion, fecha_creacion, estado, tipo_solicitud
		) VALUES ($1, $2, $3, $4, $5, 'pendiente', 'modificacion')
		RETURNING id_solicitud;
	`

	var idSolicitud int
	err := db.Pool.QueryRow(r.Context(), insertQuery,
		solicitud.IDProcesador,
		solicitud.IDPolitica,
		solicitud.DatosSolicitados,
		solicitud.Descripcion,
		time.Now(),
	).Scan(&idSolicitud)

	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			http.Error(w, "Error PostgreSQL: "+pgErr.Message, http.StatusInternalServerError)
			return
		}
		http.Error(w, "Error al insertar solicitud: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 🔍 Obtener nombre del procesador
	var nombreProcesador string
	err = db.Pool.QueryRow(context.Background(), `
		SELECT nombre FROM usuarios WHERE id_usuario = $1
	`, solicitud.IDProcesador).Scan(&nombreProcesador)
	if err != nil {
		nombreProcesador = "un procesador"
	}

	// 🔔 Notificar a todos los controladores
	rows, err := db.Pool.Query(r.Context(), `SELECT id_usuario FROM usuarios_roles WHERE id_rol = 2`)
	if err != nil {
		http.Error(w, "Error buscando controladores", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	msg := "El procesador '" + nombreProcesador + "' ha solicitado una modificación de política."

	for rows.Next() {
		var idControlador int
		if err := rows.Scan(&idControlador); err != nil {
			continue
		}

		notif := &models.Notificacion{
			UsuarioID:       idControlador,
			Tipo:            "solicitud_atributo",
			ReferenciaTabla: "solicitudes_atributo",
			ReferenciaID:    idSolicitud,
			Mensaje:         msg,
			URLRecurso:      ptrString("/controlador/solicitudes"),
		}
		_ = CrearNotificacion(r.Context(), notif)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensaje":      "Solicitud de modificación enviada correctamente",
		"id_solicitud": idSolicitud,
		"estado":       "pendiente",
	})
}

// ActualizarSolicitudInput es la estructura que recibimos en el body
type ActualizarSolicitudInput struct {
	Estado string `json:"estado"` // 'aprobado' o 'denegado'
}

// PUT /controlador/solicitudes-atributo/{id}
// PUT /controlador/solicitudes-atributo/{id}
func ActualizarEstadoSolicitudAtributo(w http.ResponseWriter, r *http.Request) {
	// 1️⃣ Leer ID de la ruta
	vars := mux.Vars(r)
	idStr := vars["id"]
	idSolicitud, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID de solicitud inválido", http.StatusBadRequest)
		return
	}

	// 2️⃣ Decodificar body
	var input struct {
		Estado string `json:"estado"` // "aprobado" o "denegado"
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido: "+err.Error(), http.StatusBadRequest)
		return
	}
	if input.Estado != "aprobado" && input.Estado != "denegado" {
		http.Error(w, "Estado debe ser 'aprobado' o 'denegado'", http.StatusBadRequest)
		return
	}

	// 3️⃣ Obtener datos previos de la solicitud
	var (
		tipo         string
		atributo     *string
		idProcesador int
	)
	err = db.Pool.QueryRow(r.Context(), `
        SELECT tipo_solicitud, atributo, id_procesador
          FROM solicitudes_atributo
         WHERE id_solicitud = $1
    `, idSolicitud).Scan(&tipo, &atributo, &idProcesador)
	if err != nil {
		http.Error(w, "Solicitud no encontrada: "+err.Error(), http.StatusNotFound)
		return
	}

	// 4️⃣ Actualizar el estado en la base
	_, err = db.Pool.Exec(r.Context(), `
        UPDATE solicitudes_atributo
           SET estado = $1
         WHERE id_solicitud = $2
    `, input.Estado, idSolicitud)
	if err != nil {
		http.Error(w, "Error actualizando estado: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5️⃣ Si era NUEVO y se APRUEBA, agregar atributo
	if tipo == "nuevo" && input.Estado == "aprobado" && atributo != nil {
		// Intentar leer fila existente
		var actuales []string
		err := db.Pool.QueryRow(r.Context(), `
            SELECT atributos
              FROM atributos_terceros
             WHERE id_usuario = $1
        `, idProcesador).Scan(&actuales)

		if err == nil {
			// Ya existe: añadir si no está
			existe := false
			for _, a := range actuales {
				if a == *atributo {
					existe = true
					break
				}
			}
			if !existe {
				actuales = append(actuales, *atributo)
				_, err = db.Pool.Exec(r.Context(), `
                    UPDATE atributos_terceros
                       SET atributos = $1,
                           fecha_asignacion = $2
                     WHERE id_usuario = $3
                `, actuales, time.Now(), idProcesador)
				if err != nil {
					http.Error(w, "Error actualizando atributos: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}
		} else {
			// No existía: insertar nueva fila
			_, err = db.Pool.Exec(r.Context(), `
                INSERT INTO atributos_terceros
                  (id_usuario, atributos, asignado_por, fecha_asignacion)
                VALUES ($1, $2, $3, $4)
            `, idProcesador, []string{*atributo} /* asignado_por= */, 1, time.Now())
			if err != nil {
				http.Error(w, "Error insertando atributos: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// 6️⃣ Enviar notificación al procesador con el resultado
	msg := fmt.Sprintf("Tu solicitud #%d ha sido %s.", idSolicitud, input.Estado)
	notif := &models.Notificacion{
		UsuarioID:       idProcesador,
		Tipo:            "resultado_solicitud",
		ReferenciaTabla: "solicitudes_atributo",
		ReferenciaID:    idSolicitud,
		Mensaje:         msg,
		URLRecurso:      ptrString("/procesador/solicitudes"),
	}
	_ = CrearNotificacion(r.Context(), notif) // ignorar error aquí

	// 7️⃣ Responder sin contenido
	w.WriteHeader(http.StatusNoContent)
}

func ObtenerAtributosDesPolitica(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id_politica")
	if idStr == "" {
		http.Error(w, "Falta el parámetro id_politica", http.StatusBadRequest)
		return
	}

	rows, err := db.Pool.Query(r.Context(), `
		SELECT ad.nombre
		FROM politica_atributo pa
		JOIN atributos_datos ad ON pa.id_atributo = ad.id_atributo
		WHERE pa.id_politica = $1
	`, idStr)
	if err != nil {
		http.Error(w, "Error consultando atributos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var atributos []map[string]string
	for rows.Next() {
		var nombre string
		if err := rows.Scan(&nombre); err != nil {
			http.Error(w, "Error leyendo resultados", http.StatusInternalServerError)
			return
		}
		atributos = append(atributos, map[string]string{"nombre": nombre})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(atributos)
}

func ObtenerSolicitudAtributoPorID(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var s SolicitudAtributo
	err = db.Pool.QueryRow(r.Context(), `
		SELECT id_solicitud, id_procesador, id_politica,
		       atributo, descripcion, fecha_creacion,
		       estado, datos_solicitados, tipo_solicitud
		FROM solicitudes_atributo
		WHERE id_solicitud = $1
	`, id).Scan(
		&s.IDSolicitud,
		&s.IDProcesador,
		&s.IDPolitica,
		&s.Atributo,
		&s.Descripcion,
		&s.FechaCreacion,
		&s.Estado,
		&s.DatosSolicitados,
		&s.TipoSolicitud,
	)
	if err != nil {
		http.Error(w, "Solicitud no encontrada: "+err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(s)
}
