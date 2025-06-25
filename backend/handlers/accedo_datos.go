// backend/handlers/acceso_datos.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"backend/db"
	"backend/models"
	"backend/utils"
)

// GET /procesador/acceso-datos?id_usuario=NN
func ObtenerAccesoDatos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1️⃣ Leer id_usuario (titular)
	titularStr := r.URL.Query().Get("id_usuario")
	if titularStr == "" {
		http.Error(w, "Falta id_usuario", http.StatusBadRequest)
		return
	}
	idTitular, err := strconv.Atoi(titularStr)
	if err != nil {
		http.Error(w, "id_usuario inválido", http.StatusBadRequest)
		return
	}

	// 2️⃣ Leer X-User-ID (solicitante/procesador)
	solicitanteStr := r.Header.Get("X-User-ID")
	if solicitanteStr == "" {
		http.Error(w, "Falta X-User-ID", http.StatusUnauthorized)
		return
	}
	idSolicitante, err := strconv.Atoi(solicitanteStr)
	if err != nil {
		http.Error(w, "X-User-ID inválido", http.StatusBadRequest)
		return
	}

	// 4️⃣ Obtenemos id_consentimiento, id_politica y la expiración del consentimiento activo más reciente
	var (
		idConsentimiento int
		idPolitica       int
		fechaExp         time.Time
	)
	err = db.Pool.
		QueryRow(ctx, `
			SELECT c.id_consentimiento, c.id_politica, c.fecha_expiracion
			  FROM consentimientos c
			 WHERE c.id_usuario      = $1
			   AND c.estado          = 'activo'
			   AND c.fecha_expiracion > NOW()
			 ORDER BY c.fecha_expiracion DESC
			 LIMIT 1
		`, idTitular,
		).
		Scan(&idConsentimiento, &idPolitica, &fechaExp)
	if err != nil {
		// No hay consentimiento activo
		LogAcceso(ctx, idSolicitante, 0, false, "no hay consentimiento activo")
		http.Error(w, "No hay consentimiento activo", http.StatusNotFound)
		return
	}

	// 3️⃣ Si no es el titular, verificar dinámico
	if idSolicitante != idTitular {
		if ok := utils.VerificarAccesoDinamico(idSolicitante, idTitular); !ok {
			LogAcceso(ctx, idSolicitante, idConsentimiento, false, "política no coincide")
			http.Error(w, "Acceso denegado según política", http.StatusForbidden)
			return
		}
	}

	// 5️⃣ Leemos los atributos permitidos para esa política
	attrRows, err := db.Pool.Query(ctx, `
		SELECT ad.nombre
		  FROM politica_atributo pa
		  JOIN atributos_datos ad ON ad.id_atributo = pa.id_atributo
		 WHERE pa.id_politica = $1
	`, idPolitica)
	if err != nil {
		LogAcceso(ctx, idSolicitante, idConsentimiento, false, "error lectura atributos")
		http.Error(w, "Error leyendo atributos de política", http.StatusInternalServerError)
		return
	}
	defer attrRows.Close()

	permitidos := make([]string, 0, 8)
	for attrRows.Next() {
		var nombre string
		if err := attrRows.Scan(&nombre); err == nil {
			permitidos = append(permitidos, nombre)
		}
	}

	// 6️⃣ Leemos el email del titular
	var email string
	err = db.Pool.
		QueryRow(ctx, `SELECT email FROM usuarios WHERE id_usuario = $1`, idTitular).
		Scan(&email)
	if err != nil {
		LogAcceso(ctx, idSolicitante, idConsentimiento, false, "titular no encontrado")
		http.Error(w, "Titular no encontrado", http.StatusNotFound)
		return
	}

	// 7️⃣ Recuperamos los datos cifrados del titular
	var dp models.DatosPersonales
	err = db.ConnDatos.
		QueryRow(ctx, `
			SELECT id_dato, telefono, celular, direccion, ciudad,
				   provincia, fecha_nacimiento, genero, estado_civil
			  FROM datos_personales
			 WHERE id_usuario = $1
		`, idTitular).
		Scan(
			&dp.IDDato,
			&dp.Telefono,
			&dp.Celular,
			&dp.Direccion,
			&dp.Ciudad,
			&dp.Provincia,
			&dp.FechaNacimiento,
			&dp.Genero,
			&dp.EstadoCivil,
		)
	if err != nil {
		LogAcceso(ctx, idSolicitante, idConsentimiento, false, "datos personales no encontrados")
		http.Error(w, "Datos personales no encontrados", http.StatusNotFound)
		return
	}

	// 8️⃣ Helper para descifrar un campo con la clave del procesador
	descifrar := func(ciphBytes []byte) string {
		ciph, err := utils.DeserializarCipher(ciphBytes)
		if err != nil {
			return "error deserializar"
		}
		plain, err := utils.DescifrarDatoABEConClaveUsuario(ciph, idSolicitante)
		if err != nil {
			return "no autorizado"
		}
		return plain
	}

	// 9️⃣ Construir la respuesta JSON
	respuesta := map[string]interface{}{
		"email":        email,
		"acceso_hasta": fechaExp.Format("2006-01-02"),
	}
	for _, campo := range permitidos {
		switch campo {
		case "telefono":
			respuesta["telefono"] = descifrar(dp.Telefono)
		case "celular":
			respuesta["celular"] = descifrar(dp.Celular)
		case "direccion":
			respuesta["direccion"] = descifrar(dp.Direccion)
		case "ciudad":
			respuesta["ciudad"] = descifrar(dp.Ciudad)
		case "provincia":
			respuesta["provincia"] = descifrar(dp.Provincia)
		case "fecha_nacimiento":
			respuesta["fecha_nacimiento"] = descifrar(dp.FechaNacimiento)
		case "genero":
			respuesta["genero"] = descifrar(dp.Genero)
		case "estado_civil":
			respuesta["estado_civil"] = descifrar(dp.EstadoCivil)
		}
	}

	// 🔟 Registrar acceso exitoso
	LogAcceso(ctx, idSolicitante, idConsentimiento, true, "Autorizado")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(respuesta)
}

// GET /procesador/titulares-por-atributo?atributo=XYZ
func ObtenerTitularesPorAtributo(w http.ResponseWriter, r *http.Request) {
	atributo := r.URL.Query().Get("atributo")
	if atributo == "" {
		http.Error(w, "Falta atributo", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	rows, err := db.Pool.Query(ctx, `
		SELECT u.id_usuario, u.email
		  FROM usuarios u
		  JOIN consentimientos c ON u.id_usuario = c.id_usuario
		  JOIN politicas_privacidad p ON c.id_politica = p.id_politica
		 WHERE p.titulo = $1
		   AND c.estado = 'activo'
		   AND c.fecha_expiracion > NOW()
	`, atributo)
	if err != nil {
		http.Error(w, "Error en consulta", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Titular struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
	}

	var lista []Titular
	for rows.Next() {
		var t Titular
		if err := rows.Scan(&t.ID, &t.Email); err == nil {
			lista = append(lista, t)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lista)
}
