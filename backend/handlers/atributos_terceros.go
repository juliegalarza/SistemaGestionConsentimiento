// backend/handlers/atributos_terceros.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"backend/db"
	"backend/models"
	"backend/utils"

	"github.com/gorilla/mux"
)

// POST /atributos-terceros
func AsignarAtributosTercero(w http.ResponseWriter, r *http.Request) {
	var attr models.AtributoTercero
	if err := json.NewDecoder(r.Body).Decode(&attr); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	// Insertar en atributos_terceros
	_, err := db.Pool.Exec(context.Background(), `
		INSERT INTO atributos_terceros (id_usuario, atributos, fecha_asignacion, asignado_por)
		VALUES ($1, $2, $3, $4)
	`, attr.IDUsuario, attr.Atributos, time.Now(), attr.AsignadoPor)
	if err != nil {
		http.Error(w, "Error al asignar atributos", http.StatusInternalServerError)
		return
	}

	// Guardar atributos como "clave" en claves_abe_usuario (como []string JSON)
	if err := utils.GuardarAtributosUsuario(attr.IDUsuario, attr.Atributos); err != nil {
		http.Error(w, "Error al guardar atributos en clave ABE", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"mensaje": "Atributos asignados correctamente",
	})
}

// GET /atributos-terceros/por-atributo?id_usuario=3&atributo=Marketing
func VerificarAtributoIndividual(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id_usuario")
	atributo := r.URL.Query().Get("atributo")
	if idStr == "" || atributo == "" {
		http.Error(w, "Faltan parámetros", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var existe bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM atributos_terceros
			WHERE id_usuario = $1 AND atributos @> $2::jsonb
		)
	`
	attrJSON := fmt.Sprintf(`["%s"]`, atributo)
	if err := db.Pool.QueryRow(context.Background(), query, id, attrJSON).Scan(&existe); err != nil {
		http.Error(w, "Error en la verificación", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"atributo": atributo,
		"acceso":   existe,
	})
}

// GET /atributos-terceros?id_usuario=3
func ObtenerAtributosDeTercero(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id_usuario")
	if idStr == "" {
		http.Error(w, "Falta id_usuario", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var atributos []string
	err = db.Pool.QueryRow(context.Background(), `
		SELECT atributos
		  FROM atributos_terceros
		 WHERE id_usuario = $1
	  ORDER BY fecha_asignacion DESC
		 LIMIT 1
	`, id).Scan(&atributos)

	if err != nil {
		// Si no hay registros, devolvemos lista vacía
		atributos = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"atributos": atributos,
	})
}

func ObtenerUsuarioPorID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	var nombre string
	err = db.Pool.QueryRow(context.Background(), `
		SELECT nombre
		  FROM usuarios
		 WHERE id_usuario = $1
	`, id).Scan(&nombre)

	if err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id_usuario": id,
		"nombre":     nombre,
	})
}

// PUT /atributos-terceros
func ActualizarAtributosTercero(w http.ResponseWriter, r *http.Request) {
	var input models.AtributoTercero
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	_, err := db.Pool.Exec(context.Background(), `
		UPDATE atributos_terceros
		   SET atributos        = $1,
			   fecha_asignacion = $2,
			   asignado_por     = $3
		 WHERE id_usuario       = $4
	`, input.Atributos, time.Now(), input.AsignadoPor, input.IDUsuario)
	if err != nil {
		http.Error(w, "Error al actualizar atributos", http.StatusInternalServerError)
		return
	}

	// Refrescar en clave ABE
	if err := utils.GuardarAtributosUsuario(input.IDUsuario, input.Atributos); err != nil {
		http.Error(w, "Error al guardar atributos en clave ABE", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"mensaje": "Atributos actualizados correctamente",
	})
}

// DELETE /atributos-terceros?id_usuario=3
func EliminarAtributosTercero(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id_usuario")
	if idStr == "" {
		http.Error(w, "Falta id_usuario", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if _, err := db.Pool.Exec(context.Background(), `
		DELETE FROM atributos_terceros WHERE id_usuario = $1
	`, id); err != nil {
		http.Error(w, "Error al eliminar atributos", http.StatusInternalServerError)
		return
	}

	if _, err := db.Pool.Exec(context.Background(), `
		DELETE FROM claves_abe_usuario WHERE id_usuario = $1
	`, id); err != nil {
		http.Error(w, "Error al eliminar clave ABE", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"mensaje": "Atributos eliminados correctamente",
	})
}
