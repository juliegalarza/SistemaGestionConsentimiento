// backend/handlers/registro.go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"backend/db"
	"backend/models"
	"backend/utils"

	"github.com/lib/pq"
)

// RegistroRequest es el payload de POST /registro
type RegistroRequest struct {
	Nombre            string `json:"nombre"`
	Email             string `json:"email"`
	Password          string `json:"password"`
	AsignarRolTitular bool   `json:"asignar_rol_titular"` // Nuevo campo
}

// RegistrarUsuario maneja POST /registro
func RegistrarUsuario(w http.ResponseWriter, r *http.Request) {
	var req RegistroRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	// 1) Generar salt + hash
	salt, err := utils.GenerarSalt()
	if err != nil {
		http.Error(w, "Error al generar salt", http.StatusInternalServerError)
		return
	}
	hash := utils.HashConSalt(req.Password, salt)

	// 2) Insertar usuario y obtener ID
	var userID int
	err = db.Pool.QueryRow(context.Background(),
		`INSERT INTO usuarios (nombre, email, fecha_registro)
		   VALUES ($1, $2, NOW())
		   RETURNING id_usuario`,
		req.Nombre, req.Email,
	).Scan(&userID)
	if err != nil {
		// si el email ya existía
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			http.Error(w, "Email ya registrado", http.StatusConflict)
			return
		}
		http.Error(w, "Error al crear usuario", http.StatusInternalServerError)
		return
	}

	// 3) Guardar credenciales
	_, err = db.Pool.Exec(context.Background(),
		`INSERT INTO credenciales_usuarios
		   (id_usuario, hash_password, salt, fecha_creacion, ultimo_acceso)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, hash, salt, time.Now(), time.Now(),
	)
	if err != nil {
		http.Error(w, "Error al guardar credenciales", http.StatusInternalServerError)
		return
	}

	// 4) Si corresponde, asignar rol de Titular
	if req.AsignarRolTitular {
		_, err = db.Pool.Exec(context.Background(),
			`INSERT INTO usuarios_roles (id_usuario, id_rol, fecha_asignacion)
			 VALUES ($1, 1, NOW())`, // Rol 1 = Titular de los datos
			userID,
		)
		if err != nil {
			http.Error(w, "Error al asignar rol de titular", http.StatusInternalServerError)
			return
		}
	}

	// 5) Responder con el nuevo ID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensaje":    "Usuario registrado correctamente",
		"id_usuario": userID,
	})
}

// ObtenerUsuarios maneja GET /usuarios
func ObtenerUsuarios(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(
		context.Background(),
		`SELECT id_usuario, nombre, email
		   FROM usuarios
		  ORDER BY id_usuario`,
	)
	if err != nil {
		http.Error(w, "Error al consultar usuarios", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lista []models.Usuario
	for rows.Next() {
		var u models.Usuario
		if err := rows.Scan(&u.ID, &u.Nombre, &u.Email); err != nil {
			http.Error(w, "Error leyendo usuario", http.StatusInternalServerError)
			return
		}
		lista = append(lista, u)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Error iterando resultados", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}
