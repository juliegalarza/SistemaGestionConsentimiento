package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"

	"backend/db"
	"backend/utils"
)

// LoginRequest es el payload de POST /login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginUsuario maneja POST /login
func LoginUsuario(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	// 1) Buscar id_usuario
	var userID int
	err := db.Pool.QueryRow(context.Background(),
		`SELECT id_usuario
		   FROM usuarios
		  WHERE email=$1`, req.Email,
	).Scan(&userID)
	if err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusUnauthorized)
		return
	}

	// 2) Traer salt y hash
	var salt, hashGuardado []byte
	err = db.Pool.QueryRow(context.Background(),
		`SELECT salt, hash_password
		   FROM credenciales_usuarios
		  WHERE id_usuario=$1`,
		userID,
	).Scan(&salt, &hashGuardado)
	if err != nil {
		http.Error(w, "Credenciales no encontradas", http.StatusUnauthorized)
		return
	}

	// 3) Comparar hash
	hashIngresado := utils.HashConSalt(req.Password, salt)
	if !bytes.Equal([]byte(hashIngresado), hashGuardado) {
		http.Error(w, "Contraseña incorrecta", http.StatusUnauthorized)
		return
	}

	// 4) Actualizar último acceso
	if _, err := db.Pool.Exec(context.Background(),
		`UPDATE credenciales_usuarios
		    SET ultimo_acceso = NOW()
		  WHERE id_usuario = $1`,
		userID,
	); err != nil {
		http.Error(w, "Error al actualizar último acceso", http.StatusInternalServerError)
		return
	}

	// 5) Auditoría (opcional)
	if _, err := db.Pool.Exec(context.Background(),
		`INSERT INTO auditoria_login(id_usuario, exito, fecha_intento)
		 VALUES($1, TRUE, NOW())`,
		userID,
	); err != nil {
		log.Println("Error auditando login:", err)
	}

	// 6) Traer el id_rol
	var idRol int
	err = db.Pool.QueryRow(context.Background(),
		`SELECT id_rol
		   FROM usuarios_roles
		  WHERE id_usuario = $1
		  LIMIT 1`, // En caso de que algún usuario tenga varios roles (solo tomamos uno aquí)
		userID,
	).Scan(&idRol)
	if err != nil {
		http.Error(w, "No se pudo obtener el rol del usuario", http.StatusInternalServerError)
		return
	}

	// 7) Responder con el ID y el rol
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensaje":    "Inicio de sesión exitoso",
		"id_usuario": userID,
		"id_rol":     idRol,
	})
}
