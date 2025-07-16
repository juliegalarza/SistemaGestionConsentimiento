// src/backend/handlers/roles.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"backend/db"
)

// --- Tipos de entrada y salida ---

type AsignarRolInput struct {
	IDUsuario int `json:"id_usuario"`
	IDRol     int `json:"id_rol"`
}

type Rol struct {
	ID     int    `json:"id_rol"`
	Nombre string `json:"nombre"`
}

// UsuarioAsignable es la estructura que devolvemos al front para poblar la tabla.
// Si el usuario ya tiene rol (distinto de 1 y 2), traerá IDRolActual y FechaAsignacion;
// si no, ambos campos serán nil y Estado = "Pendiente".
type UsuarioAsignable struct {
	IDUsuario       int        `json:"id_usuario"`
	Nombre          string     `json:"nombre"`
	Email           string     `json:"email"`
	IDRolActual     *int       `json:"id_rol_actual"`
	FechaAsignacion *time.Time `json:"fecha_asignacion"`
	Estado          string     `json:"estado"` // "Pendiente" o "Asignado"
}

// --- Endpoints existentes ---

// ObtenerRoles devuelve todos los roles disponibles.
func ObtenerRoles(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(r.Context(), `
		SELECT id_rol, nombre
		  FROM roles
		ORDER BY id_rol
	`)
	if err != nil {
		http.Error(w, "Error al obtener roles: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lista []Rol
	for rows.Next() {
		var item Rol
		if err := rows.Scan(&item.ID, &item.Nombre); err != nil {
			continue
		}
		lista = append(lista, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}

// CrearUsuarioRol inserta o actualiza la asignación de rol para un usuario.
func CrearUsuarioRol(w http.ResponseWriter, r *http.Request) {
	var input AsignarRolInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	_, err := db.Pool.Exec(r.Context(), `
		INSERT INTO usuarios_roles (id_usuario, id_rol, fecha_asignacion)
		VALUES ($1, $2, $3)
		ON CONFLICT (id_usuario)
		  DO UPDATE
		    SET id_rol          = EXCLUDED.id_rol,
		        fecha_asignacion = EXCLUDED.fecha_asignacion
	`, input.IDUsuario, input.IDRol, time.Now())
	if err != nil {
		http.Error(w, "Error al asignar rol", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Rol asignado correctamente"})
}

// ObtenerRolesUsuario devuelve los roles que ya tiene un usuario específico.
func ObtenerRolesUsuario(w http.ResponseWriter, r *http.Request) {
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

	rows, err := db.Pool.Query(r.Context(), `
		SELECT ur.id_usuario, ur.id_rol, r.nombre, ur.fecha_asignacion
		  FROM usuarios_roles ur
		  JOIN roles r ON ur.id_rol = r.id_rol
		 WHERE ur.id_usuario = $1
	`, id)
	if err != nil {
		http.Error(w, "Error en la consulta", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var resultado []map[string]interface{}
	for rows.Next() {
		var idUsuario, idRol int
		var nombreRol string
		var fecha time.Time
		if err := rows.Scan(&idUsuario, &idRol, &nombreRol, &fecha); err != nil {
			continue
		}
		resultado = append(resultado, map[string]interface{}{
			"id_usuario":       idUsuario,
			"id_rol":           idRol,
			"rol":              nombreRol,
			"fecha_asignacion": fecha.Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Error al leer datos", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resultado)
}

// EliminarUsuarioRol revoca un rol concreto de un usuario.
func EliminarUsuarioRol(w http.ResponseWriter, r *http.Request) {
	idUsr := r.URL.Query().Get("id_usuario")
	idRol := r.URL.Query().Get("id_rol")
	if idUsr == "" || idRol == "" {
		http.Error(w, "Faltan parámetros", http.StatusBadRequest)
		return
	}
	u, err1 := strconv.Atoi(idUsr)
	v, err2 := strconv.Atoi(idRol)
	if err1 != nil || err2 != nil {
		http.Error(w, "Parámetros inválidos", http.StatusBadRequest)
		return
	}

	_, err := db.Pool.Exec(r.Context(), `
		DELETE FROM usuarios_roles
		 WHERE id_usuario = $1
		   AND id_rol     = $2
	`, u, v)
	if err != nil {
		http.Error(w, "Error al eliminar rol del usuario", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensaje": "Rol revocado correctamente"})
}

// ObtenerUsuariosSinRol lista a quienes NO tengan rol 1 ni 2.
func ObtenerUsuariosSinRol(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(r.Context(), `
		SELECT u.id_usuario, u.nombre, u.email
		  FROM usuarios u
		 WHERE NOT EXISTS (
		     SELECT 1
		       FROM usuarios_roles ur
		      WHERE ur.id_usuario = u.id_usuario
		        AND ur.id_rol IN (1,2)
		 )
		 ORDER BY u.id_usuario
	`)
	if err != nil {
		http.Error(w, "Error al obtener usuarios sin rol: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id int
		var nombre, email string
		if err := rows.Scan(&id, &nombre, &email); err != nil {
			continue
		}
		list = append(list, map[string]interface{}{
			"id_usuario": id,
			"nombre":     nombre,
			"email":      email,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// ObtenerUsuariosProcesadores devuelve los usuarios con rol "Procesador de los datos".
func ObtenerUsuariosProcesadores(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(r.Context(), `
		SELECT u.id_usuario, u.nombre, u.email, r.nombre AS rol
		  FROM usuarios u
		  JOIN usuarios_roles ur ON u.id_usuario = ur.id_usuario
		  JOIN roles r ON ur.id_rol = r.id_rol
		 WHERE r.nombre = 'Procesador de los datos'
	`)
	if err != nil {
		http.Error(w, "Error al obtener usuarios procesadores: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var usuarios []map[string]interface{}
	for rows.Next() {
		var idUsuario int
		var nombre, email, rol string
		if err := rows.Scan(&idUsuario, &nombre, &email, &rol); err != nil {
			continue
		}
		usuarios = append(usuarios, map[string]interface{}{
			"id_usuario": idUsuario,
			"nombre":     nombre,
			"email":      email,
			"rol":        rol,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usuarios)
}

// --- Nuevo endpoint que solicitaste: lista TODOS los usuarios junto a su estado de rol —pendiente o asignado— ---
func ObtenerUsuariosAsignables(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(r.Context(), `
		SELECT
		  u.id_usuario,
		  u.nombre,
		  u.email,
		  ur.id_rol          AS id_rol_actual,
		  ur.fecha_asignacion
		FROM usuarios u
		LEFT JOIN usuarios_roles ur
		  ON u.id_usuario = ur.id_usuario
		  AND ur.id_rol NOT IN (1,2)
		ORDER BY u.id_usuario
	`)
	if err != nil {
		http.Error(w, "Error al obtener asignables: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lista []UsuarioAsignable
	for rows.Next() {
		var x UsuarioAsignable
		if err := rows.Scan(
			&x.IDUsuario,
			&x.Nombre,
			&x.Email,
			&x.IDRolActual,
			&x.FechaAsignacion,
		); err != nil {
			continue
		}
		// Rellenar Estado según tenga rol o no
		if x.IDRolActual != nil {
			x.Estado = "Asignado"
		} else {
			x.Estado = "Pendiente"
		}
		lista = append(lista, x)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Error iterando asignables: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}
