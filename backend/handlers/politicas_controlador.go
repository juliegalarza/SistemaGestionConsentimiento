package handlers

import (
	"backend/db"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

func auditPoliticaFailure(ctx context.Context, operacion, descripcion string, idPolitica int, errMsg string) {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO auditoria_politicas
		  (operacion, id_politica, titulo, descripcion, fecha, usuario, exito, error_mensaje)
		VALUES (
		  $1, $2,
		  (SELECT titulo FROM politicas_privacidad WHERE id_politica=$2),
		  $3, NOW(),
		  current_setting('app.current_user')::text,
		  false, $4
		)
	`, operacion, idPolitica, descripcion, errMsg)
	if err != nil {
		log.Printf("Error auditando fallo política [%s %d]: %v", operacion, idPolitica, err)
	}
}

func ObtenerPoliticasParaControlador(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := db.Pool.Query(ctx, `
        SELECT id_politica, titulo, descripcion, fecha_inicio, fecha_fin
          FROM politicas_privacidad
      ORDER BY id_politica
    `)
	if err != nil {
		http.Error(w, "Error al obtener políticas: "+err.Error(), http.StatusInternalServerError)
		auditPoliticaFailure(ctx, "SELECT", "Obtener listado de políticas", 0, err.Error())
		return
	}
	defer rows.Close()

	type Politica struct {
		ID          int       `json:"id_politica"`
		Titulo      string    `json:"titulo"`
		Descripcion string    `json:"descripcion"`
		FechaInicio time.Time `json:"fecha_inicio"`
		FechaFin    time.Time `json:"fecha_fin"`
	}

	var lista []Politica
	for rows.Next() {
		var p Politica
		if err := rows.Scan(&p.ID, &p.Titulo, &p.Descripcion, &p.FechaInicio, &p.FechaFin); err != nil {
			continue
		}
		lista = append(lista, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}

func ObtenerAtributosDatos(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(context.Background(), `
		SELECT id_atributo, nombre, etiqueta FROM atributos_datos
	`)
	if err != nil {
		http.Error(w, "Error al consultar atributos de datos", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var resultados []map[string]interface{}
	for rows.Next() {
		var id int
		var nombre, etiqueta string
		if err := rows.Scan(&id, &nombre, &etiqueta); err != nil {
			continue
		}
		resultados = append(resultados, map[string]interface{}{
			"id_atributo": id,
			"nombre":      nombre,
			"etiqueta":    etiqueta,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resultados)
}

func CrearPoliticaControlador(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type input struct {
		Titulo      string `json:"titulo"`
		Descripcion string `json:"descripcion"`
		FechaInicio string `json:"fecha_inicio"`
		FechaFin    string `json:"fecha_fin"`
		Atributos   []int  `json:"atributos"`
	}
	var in input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		http.Error(w, "Error iniciando transacción", http.StatusInternalServerError)
		auditPoliticaFailure(ctx, "BEGIN", "Crear política", 0, err.Error())
		return
	}
	defer tx.Rollback(ctx)

	var idPol int
	err = tx.QueryRow(ctx, `
		INSERT INTO politicas_privacidad (titulo, descripcion, fecha_inicio, fecha_fin)
		VALUES ($1,$2,$3,$4)
		RETURNING id_politica
	`, in.Titulo, in.Descripcion, in.FechaInicio, in.FechaFin).Scan(&idPol)
	if err != nil {
		http.Error(w, "Error insertando política: "+err.Error(), http.StatusInternalServerError)
		auditPoliticaFailure(ctx, "INSERT", fmt.Sprintf("Crear '%s'", in.Titulo), 0, err.Error())
		return
	}

	for _, aid := range in.Atributos {
		if _, err := tx.Exec(ctx, `
			INSERT INTO politica_atributo (id_politica, id_atributo)
			VALUES ($1,$2)
		`, idPol, aid); err != nil {
			http.Error(w, "Error asociando atributos: "+err.Error(), http.StatusInternalServerError)
			auditPoliticaFailure(ctx, "INSERT_ATTR", fmt.Sprintf("Asociar atributo %d", aid), idPol, err.Error())
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Error guardando política", http.StatusInternalServerError)
		auditPoliticaFailure(ctx, "COMMIT", "Crear política", idPol, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensaje":     "Política creada correctamente",
		"id_politica": idPol,
	})
}

func ActualizarPoliticaControlador(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := mux.Vars(r)["id_politica"]
	idPol, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	type input struct {
		Titulo      string `json:"titulo"`
		Descripcion string `json:"descripcion"`
		FechaInicio string `json:"fecha_inicio"`
		FechaFin    string `json:"fecha_fin"`
		Atributos   []int  `json:"atributos"`
	}
	var in input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		http.Error(w, "Error iniciando transacción", http.StatusInternalServerError)
		auditPoliticaFailure(ctx, "BEGIN", "Actualizar política", idPol, err.Error())
		return
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE politicas_privacidad
		   SET titulo=$1, descripcion=$2, fecha_inicio=$3, fecha_fin=$4
		 WHERE id_politica=$5
	`, in.Titulo, in.Descripcion, in.FechaInicio, in.FechaFin, idPol); err != nil {
		http.Error(w, "Error actualizando política: "+err.Error(), http.StatusInternalServerError)
		auditPoliticaFailure(ctx, "UPDATE", "Actualizar campos", idPol, err.Error())
		return
	}

	if _, err := tx.Exec(ctx, `DELETE FROM politica_atributo WHERE id_politica=$1`, idPol); err != nil {
		http.Error(w, "Error eliminando atributos anteriores: "+err.Error(), http.StatusInternalServerError)
		auditPoliticaFailure(ctx, "DELETE_ATTR", "Eliminar atributos previos", idPol, err.Error())
		return
	}

	for _, aid := range in.Atributos {
		if _, err := tx.Exec(ctx, `
			INSERT INTO politica_atributo (id_politica, id_atributo)
			VALUES ($1,$2)
		`, idPol, aid); err != nil {
			http.Error(w, "Error insertando atributos nuevos: "+err.Error(), http.StatusInternalServerError)
			auditPoliticaFailure(ctx, "INSERT_ATTR", fmt.Sprintf("Asociar atributo %d", aid), idPol, err.Error())
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Error guardando cambios", http.StatusInternalServerError)
		auditPoliticaFailure(ctx, "COMMIT", "Actualizar política", idPol, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Política actualizada correctamente"})
}

func ObtenerAtributosDePolitica(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id_politica")
	id, err := strconv.Atoi(idStr)
	if idStr == "" || err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	rows, err := db.Pool.Query(context.Background(), `
		SELECT id_atributo FROM politica_atributo WHERE id_politica = $1
	`, id)
	if err != nil {
		http.Error(w, "Error consultando atributos", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var atributos []int
	for rows.Next() {
		var idAtributo int
		if err := rows.Scan(&idAtributo); err == nil {
			atributos = append(atributos, idAtributo)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(atributos)
}

func ObtenerPoliticaPorIDC(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := mux.Vars(r)["id_politica"]
	idPol, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var p struct {
		ID          int       `json:"id_politica"`
		Titulo      string    `json:"titulo"`
		Descripcion string    `json:"descripcion"`
		FechaInicio time.Time `json:"fecha_inicio"`
		FechaFin    time.Time `json:"fecha_fin"`
	}
	if err := db.Pool.QueryRow(ctx, `
		SELECT id_politica, titulo, descripcion, fecha_inicio, fecha_fin
		  FROM politicas_privacidad
		 WHERE id_politica=$1
	`, idPol).Scan(&p.ID, &p.Titulo, &p.Descripcion, &p.FechaInicio, &p.FechaFin); err != nil {
		http.Error(w, "No se encontró la política", http.StatusNotFound)
		auditPoliticaFailure(ctx, "SELECT", "Obtener política por ID", idPol, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func ObtenerConsentimientos(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(context.Background(), `
		SELECT c.id_consentimiento, u.nombre, p.titulo, c.fecha_expiracion, c.estado
		FROM consentimientos c
		JOIN usuarios u ON u.id_usuario = c.id_usuario
		JOIN politicas_privacidad p ON p.id_politica = c.id_politica
	`)
	if err != nil {
		http.Error(w, "Error al obtener consentimientos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lista []map[string]interface{}
	for rows.Next() {
		var id int
		var nombre, titulo, estado string
		var fechaExp *time.Time
		if err := rows.Scan(&id, &nombre, &titulo, &fechaExp, &estado); err == nil {
			item := map[string]interface{}{
				"id_consentimiento": id,
				"usuario":           nombre,
				"politica":          titulo,
				"estado":            estado,
				"fecha_expiracion":  fechaExp,
			}
			lista = append(lista, item)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}

func GenerarNotificacionesConsentimientos() error {
	ctx := context.Background()

	rows, err := db.Pool.Query(ctx, `
		SELECT c.id_consentimiento, c.id_usuario, c.id_politica, c.fecha_expiracion, c.estado, p.titulo
		FROM consentimientos c
		JOIN politicas_privacidad p ON c.id_politica = p.id_politica
		WHERE c.estado = 'activo' AND c.fecha_expiracion IS NOT NULL
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	today := time.Now()

	for rows.Next() {
		var idConsent, idUsuario, idPolitica int
		var fechaExp time.Time
		var estado, titulo string

		if err := rows.Scan(&idConsent, &idUsuario, &idPolitica, &fechaExp, &estado, &titulo); err != nil {
			continue
		}

		dias := int(fechaExp.Sub(today).Hours() / 24)

		if dias == 3 {
			// notificar usuario y procesadores
			_ = registrarNotificacion(idUsuario, "consentimiento", idConsent, "Tu consentimiento para '"+titulo+"' expirará pronto.")

			// Notificar a los procesadores que tengan ese atributo
			notificarProcesadores(idPolitica, idConsent, titulo)
		} else if today.After(fechaExp) {
			_ = registrarNotificacion(idUsuario, "consentimiento", idConsent, "Tu consentimiento para '"+titulo+"' ha expirado.")
			_ = notificarProcesadores(idPolitica, idConsent, titulo)
		}
	}

	return nil
}

func registrarNotificacion(idUsuario int, tipo string, refId int, mensaje string) error {
	_, err := db.Pool.Exec(context.Background(), `
		INSERT INTO notificaciones (id_usuario, tipo, referencia_tabla, referencia_id, mensaje, enviado_email, leido, fecha_creacion)
		VALUES ($1, $2, 'consentimientos', $3, $4, false, false, now())
	`, idUsuario, tipo, refId, mensaje)
	return err
}

func notificarProcesadores(idPolitica int, refId int, titulo string) error {
	ctx := context.Background()

	// Obtener atributo de la política
	var atributo string
	err := db.Pool.QueryRow(ctx, `
		SELECT a.nombre FROM politica_atributo pa
		JOIN atributos_datos a ON a.id_atributo = pa.id_atributo
		WHERE pa.id_politica = $1 LIMIT 1
	`, idPolitica).Scan(&atributo)
	if err != nil {
		return err
	}

	// Buscar usuarios que tengan ese atributo
	rows, err := db.Pool.Query(ctx, `
		SELECT id_usuario FROM atributos_terceros
		WHERE atributos::text LIKE '%' || $1 || '%'
	`, atributo)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var idProc int
		if err := rows.Scan(&idProc); err == nil {
			_ = registrarNotificacion(idProc, "consentimiento", refId, "El consentimiento para '"+titulo+"' expirará o ha expirado.")
		}
	}
	return nil
}
func EliminarPoliticaControlador(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := mux.Vars(r)["id_politica"]
	idPol, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if _, err := db.Pool.Exec(ctx, `
		DELETE FROM politicas_privacidad WHERE id_politica=$1
	`, idPol); err != nil {
		http.Error(w, "Error eliminando política: "+err.Error(), http.StatusInternalServerError)
		auditPoliticaFailure(ctx, "DELETE", "Eliminar política", idPol, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Política eliminada correctamente"})
}
