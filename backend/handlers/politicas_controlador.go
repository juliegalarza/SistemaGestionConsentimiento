package handlers

import (
	"backend/db"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

func ObtenerPoliticasParaControlador(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(context.Background(), `
        SELECT id_politica,
               titulo,
               descripcion,
               fecha_inicio,
               fecha_fin
          FROM politicas_privacidad
      ORDER BY id_politica
    `)
	if err != nil {
		http.Error(w, "Error al obtener políticas: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Define una struct en Go para recibir todos los campos
	type politItem struct {
		ID          int       `json:"id_politica"`
		Titulo      string    `json:"titulo"`
		Descripcion string    `json:"descripcion"`
		FechaInicio time.Time `json:"fecha_inicio"`
		FechaFin    time.Time `json:"fecha_fin"`
	}

	var lista []politItem
	for rows.Next() {
		var p politItem
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
	type InputPolitica struct {
		Titulo      string `json:"titulo"`
		Descripcion string `json:"descripcion"`
		FechaInicio string `json:"fecha_inicio"`
		FechaFin    string `json:"fecha_fin"`
		Atributos   []int  `json:"atributos"`
	}

	var input InputPolitica
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	tx, err := db.Pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Error iniciando transacción", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	var idPolitica int
	err = tx.QueryRow(context.Background(), `
		INSERT INTO politicas_privacidad (titulo, descripcion, fecha_inicio, fecha_fin)
		VALUES ($1, $2, $3, $4)
		RETURNING id_politica
	`, input.Titulo, input.Descripcion, input.FechaInicio, input.FechaFin).Scan(&idPolitica)
	if err != nil {
		http.Error(w, "Error insertando política", http.StatusInternalServerError)
		return
	}

	for _, idAtributo := range input.Atributos {
		_, err := tx.Exec(context.Background(), `
			INSERT INTO politica_atributo (id_politica, id_atributo)
			VALUES ($1, $2)
		`, idPolitica, idAtributo)
		if err != nil {
			http.Error(w, "Error asociando atributos", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		http.Error(w, "Error guardando política", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensaje":     "Política creada correctamente",
		"id_politica": idPolitica,
	})
}

func ActualizarPoliticaControlador(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id_politica"]
	idPolitica, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var input struct {
		Titulo      string `json:"titulo"`
		Descripcion string `json:"descripcion"`
		FechaInicio string `json:"fecha_inicio"`
		FechaFin    string `json:"fecha_fin"`
		Atributos   []int  `json:"atributos"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	tx, err := db.Pool.Begin(context.Background())
	if err != nil {
		http.Error(w, "Error iniciando transacción", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	_, err = tx.Exec(context.Background(), `
		UPDATE politicas_privacidad
		SET titulo = $1, descripcion = $2, fecha_inicio = $3, fecha_fin = $4
		WHERE id_politica = $5
	`, input.Titulo, input.Descripcion, input.FechaInicio, input.FechaFin, idPolitica)
	if err != nil {
		http.Error(w, "Error actualizando política: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(context.Background(), `
		DELETE FROM politica_atributo WHERE id_politica = $1
	`, idPolitica)
	if err != nil {
		http.Error(w, "Error eliminando atributos anteriores", http.StatusInternalServerError)
		return
	}

	for _, idAtributo := range input.Atributos {
		_, err := tx.Exec(context.Background(), `
			INSERT INTO politica_atributo (id_politica, id_atributo)
			VALUES ($1, $2)
		`, idPolitica, idAtributo)
		if err != nil {
			http.Error(w, "Error insertando atributos nuevos", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		http.Error(w, "Error guardando cambios", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"mensaje": "Política actualizada correctamente",
	})
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
	idStr := mux.Vars(r)["id_politica"]
	id, err := strconv.Atoi(idStr)
	if idStr == "" || err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var politica struct {
		ID          int       `json:"id_politica"`
		Titulo      string    `json:"titulo"`
		Descripcion string    `json:"descripcion"`
		FechaInicio time.Time `json:"fecha_inicio"`
		FechaFin    time.Time `json:"fecha_fin"`
	}

	err = db.Pool.QueryRow(context.Background(), `
		SELECT id_politica, titulo, descripcion, fecha_inicio, fecha_fin
		FROM politicas_privacidad
		WHERE id_politica = $1
	`, id).Scan(&politica.ID, &politica.Titulo, &politica.Descripcion, &politica.FechaInicio, &politica.FechaFin)
	if err != nil {
		http.Error(w, "No se encontró la política", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(politica)
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
	idStr := mux.Vars(r)["id_politica"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	_, err = db.Pool.Exec(context.Background(), `
		DELETE FROM politicas_privacidad WHERE id_politica = $1
	`, id)
	if err != nil {
		http.Error(w, "Error eliminando política: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"mensaje": "Política eliminada correctamente",
	})
}
