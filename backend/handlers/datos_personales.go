// backend/handlers/datos_personales.go
package handlers

import (
	"backend/db"
	"backend/models"
	"backend/utils"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// --------------------------
// Estructuras de entrada
// --------------------------

type DatosPersonalesInput struct {
	IDUsuario       int    `json:"id_usuario"`
	Telefono        string `json:"telefono"`
	Celular         string `json:"celular"`
	Direccion       string `json:"direccion"`
	Ciudad          string `json:"ciudad"`
	Provincia       string `json:"provincia"`
	FechaNacimiento string `json:"fecha_nacimiento"`
	Genero          string `json:"genero"`
	EstadoCivil     string `json:"estado_civil"`
}

// --------------------------
// Función auxiliar: Construir Política ABE
// --------------------------
/*
   Ahora obtenemos directamente los títulos de las políticas (politicas_privacidad.titulo)
   para las cuales el usuario ha dado consentimiento activo. Si no hay ninguno, devolvemos
   únicamente "owner:<ID>".

   Ejemplo:
     - Si el usuario 16 tiene un consentimiento activo con id_politica=4, y
       politicas_privacidad.titulo[4] = "Investigación",
       entonces la política será "owner:16 OR Investigación".
*/

/*func construirPoliticaDinamica(idUsuario int) (string, error) {
	// 1) Siempre incluimos "owner:<id>"
	partes := []string{fmt.Sprintf("owner:%d", idUsuario)}

	// 2) Buscamos TÍTULOS de políticas de privacidad activas aceptadas por el usuario
	//
	//    SELECT DISTINCT p.titulo
	//      FROM consentimientos c
	//      JOIN politicas_privacidad p ON c.id_politica = p.id_politica
	//     WHERE c.id_usuario = $1
	//       AND c.estado = 'activo'
	//       AND c.fecha_expiracion > NOW()
	rows, err := db.Pool.Query(context.Background(), `
		SELECT DISTINCT p.titulo
		  FROM consentimientos c
		  JOIN politicas_privacidad p ON c.id_politica = p.id_politica
		 WHERE c.id_usuario     = $1
		   AND c.estado         = 'activo'
		   AND c.fecha_expiracion > NOW()
	`, idUsuario)
	if err != nil {
		return "", fmt.Errorf("error consultando políticas activas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tituloPol string
		if err := rows.Scan(&tituloPol); err != nil {
			return "", fmt.Errorf("error leyendo título de política: %w", err)
		}
		partes = append(partes, tituloPol)
	}

	// 3) Si sólo tenemos “owner:<id>”, devolvemos únicamente eso
	if len(partes) == 1 {
		return partes[0], nil
	}

	// 4) Concatenamos con " OR "
	return strings.Join(partes, " OR "), nil
} */

func construirPoliticaDinamica(idUsuario int) (string, error) {
	// Siempre incluimos el owner
	partes := []string{fmt.Sprintf("owner:%d", idUsuario)}

	// Consulta títulos de políticas activas + pendientes de revocación (<24 h)
	const sqlQuery = `
		SELECT DISTINCT p.titulo
		  FROM consentimientos c
		  JOIN politicas_privacidad p ON c.id_politica = p.id_politica
		 WHERE c.id_usuario = $1
		   AND (
		         (c.estado = 'activo' AND c.fecha_expiracion > NOW())
		      OR (c.estado = 'revocado_pendiente'
		          AND c.fecha_revocacion > NOW() - INTERVAL '24 hours')
		   )
	`

	rows, err := db.Pool.Query(context.Background(), sqlQuery, idUsuario)
	if err != nil {
		return "", fmt.Errorf("error consultando políticas dinámicas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var titulo string
		if err := rows.Scan(&titulo); err != nil {
			return "", fmt.Errorf("error leyendo título de política: %w", err)
		}
		partes = append(partes, titulo)
	}

	// Si solo tenemos el owner, devolvemos eso
	if len(partes) == 1 {
		return partes[0], nil
	}

	// Concatenar con OR
	return strings.Join(partes, " OR "), nil
}

// --------------------------
// Handler: Guardar / Actualizar Datos Personales
// --------------------------

func GuardarDatosPersonales(w http.ResponseWriter, r *http.Request) {
	var input DatosPersonalesInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	// 1) Construir la política ABE dinámica (owner:<id> OR <títulos de políticas activas>)
	politica, err := construirPoliticaDinamica(input.IDUsuario)
	if err != nil {
		http.Error(w, "No se pudo construir la política ABE: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2) Helper para cifrar + serializar
	encrypt := func(plain string) ([]byte, error) {
		ciph, err := utils.CifrarDatoABE(plain, politica)
		if err != nil {
			return nil, err
		}
		return utils.SerializarCipher(ciph)
	}

	// 3) Cifrar cada campo
	telBytes, err := encrypt(input.Telefono)
	if err != nil {
		http.Error(w, "Error al cifrar teléfono", http.StatusInternalServerError)
		return
	}
	celBytes, err := encrypt(input.Celular)
	if err != nil {
		http.Error(w, "Error al cifrar celular", http.StatusInternalServerError)
		return
	}
	dirBytes, err := encrypt(input.Direccion)
	if err != nil {
		http.Error(w, "Error al cifrar dirección", http.StatusInternalServerError)
		return
	}
	ciuBytes, err := encrypt(input.Ciudad)
	if err != nil {
		http.Error(w, "Error al cifrar ciudad", http.StatusInternalServerError)
		return
	}
	provBytes, err := encrypt(input.Provincia)
	if err != nil {
		http.Error(w, "Error al cifrar provincia", http.StatusInternalServerError)
		return
	}
	fechaBytes, err := encrypt(input.FechaNacimiento)
	if err != nil {
		http.Error(w, "Error al cifrar fecha de nacimiento", http.StatusInternalServerError)
		return
	}
	genBytes, err := encrypt(input.Genero)
	if err != nil {
		http.Error(w, "Error al cifrar género", http.StatusInternalServerError)
		return
	}
	estadoBytes, err := encrypt(input.EstadoCivil)
	if err != nil {
		http.Error(w, "Error al cifrar estado civil", http.StatusInternalServerError)
		return
	}

	// 4) Upsert: INSERT o UPDATE según ya exista fila para este usuario
	_, err = db.ConnDatos.Exec(context.Background(), `
		INSERT INTO datos_personales
		  (id_usuario, telefono, celular, direccion, ciudad, provincia,
		   fecha_nacimiento, genero, estado_civil, fecha_creacion)
		VALUES
		  ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())
		ON CONFLICT (id_usuario) DO UPDATE
		  SET telefono         = EXCLUDED.telefono,
		      celular          = EXCLUDED.celular,
		      direccion        = EXCLUDED.direccion,
		      ciudad           = EXCLUDED.ciudad,
		      provincia        = EXCLUDED.provincia,
		      fecha_nacimiento = EXCLUDED.fecha_nacimiento,
		      genero           = EXCLUDED.genero,
		      estado_civil     = EXCLUDED.estado_civil
	`, input.IDUsuario,
		telBytes, celBytes, dirBytes, ciuBytes, provBytes,
		fechaBytes, genBytes, estadoBytes,
	)
	if err != nil {
		http.Error(w, "Error al guardar/actualizar datos personales: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5) Respuesta con la política que se usó para cifrar
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"mensaje":  "Datos personales cifrados correctamente",
		"politica": politica,
	})
}

// --------------------------
// Handler: Obtener / Descifrar Datos Personales
// --------------------------

func ObtenerDatosPersonales(w http.ResponseWriter, r *http.Request) {
	// 1) Leer id_usuario (obligatorio)
	uid := r.URL.Query().Get("id_usuario")
	if uid == "" {
		http.Error(w, "Falta id_usuario", http.StatusBadRequest)
		return
	}
	idUsuario, err := strconv.Atoi(uid)
	if err != nil {
		http.Error(w, "id_usuario inválido", http.StatusBadRequest)
		return
	}

	// 2) Leer id_solicitante (puede venir o no)
	idSolicitante := idUsuario // por defecto asumimos titular
	if sol := r.URL.Query().Get("id_solicitante"); sol != "" {
		idSol, err := strconv.Atoi(sol)
		if err != nil {
			http.Error(w, "id_solicitante inválido", http.StatusBadRequest)
			return
		}
		idSolicitante = idSol

		// Si no es el titular, verificar acceso dinámico
		if idSolicitante != idUsuario {
			if ok := utils.VerificarAccesoDinamico(idSolicitante, idUsuario); !ok {
				http.Error(w,
					"Acceso denegado: no coincide con política",
					http.StatusForbidden,
				)
				return
			}
		}
	}

	// 3) Recuperar los datos cifrados
	var datos models.DatosPersonales
	err = db.ConnDatos.QueryRow(context.Background(), `
        SELECT id_dato, id_usuario,
               telefono, celular, direccion, ciudad,
               provincia, fecha_nacimiento, genero, estado_civil, fecha_creacion
          FROM datos_personales
         WHERE id_usuario = $1
    `, idUsuario).Scan(
		&datos.IDDato,
		&datos.IDUsuario,
		&datos.Telefono,
		&datos.Celular,
		&datos.Direccion,
		&datos.Ciudad,
		&datos.Provincia,
		&datos.FechaNacimiento,
		&datos.Genero,
		&datos.EstadoCivil,
		&datos.FechaCreacion,
	)
	if err != nil {
		http.Error(w, "Datos personales no encontrados", http.StatusNotFound)
		return
	}

	// 4) Reconstruir la política ABE deseada (owner:<id> OR <títulos de políticas activas>)
	politica, err := construirPoliticaDinamica(idUsuario)
	if err != nil {
		http.Error(w, "No se pudo obtener la política activa: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5) Separar la política en claves (para el caso de titular)
	claves := strings.Split(politica, " OR ")

	// 6) Función helper para descifrar
	descifrar := func(ciphBytes []byte) string {
		ciph, err := utils.DeserializarCipher(ciphBytes)
		if err != nil {
			return "error al deserializar"
		}

		// Si es el titular, usamos la clave maestra
		if idSolicitante == idUsuario {
			plain, err := utils.DescifrarDatoABEConMaster(ciph, claves)
			if err != nil {
				return "no autorizado"
			}
			return plain
		}

		// Si es un tercero → usamos su clave persistida
		plain, err := utils.DescifrarDatoABEConClaveUsuario(ciph, idSolicitante)
		if err != nil {
			return "no autorizado"
		}
		return plain
	}

	// 7) Construir la respuesta JSON
	resp := map[string]interface{}{
		"id_dato":            datos.IDDato,
		"id_usuario":         datos.IDUsuario,
		"telefono":           descifrar(datos.Telefono),
		"celular":            descifrar(datos.Celular),
		"direccion":          descifrar(datos.Direccion),
		"ciudad":             descifrar(datos.Ciudad),
		"provincia":          descifrar(datos.Provincia),
		"fecha_nacimiento":   descifrar(datos.FechaNacimiento),
		"genero":             descifrar(datos.Genero),
		"estado_civil":       descifrar(datos.EstadoCivil),
		"fecha_creacion":     datos.FechaCreacion,
		"politica_utilizada": politica,
	}
	// 8) Registrar acceso SATISFACTORIO
	LogAcceso(r.Context(), idSolicitante, datos.IDDato, true, "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func ActualizarDatosPersonales(w http.ResponseWriter, r *http.Request) {
	var input DatosPersonalesInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Datos inválidos", http.StatusBadRequest)
		return
	}

	// 1) Reconstruir la política ABE dinámica (owner:<id> OR <títulos de políticas activas>)
	politica, err := construirPoliticaDinamica(input.IDUsuario)
	if err != nil {
		http.Error(w, "No se pudo construir la política ABE: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2) Helper para cifrar + serializar
	encrypt := func(plain string) ([]byte, error) {
		ciph, err := utils.CifrarDatoABE(plain, politica)
		if err != nil {
			return nil, err
		}
		return utils.SerializarCipher(ciph)
	}

	// 3) Cifrar cada campo
	telBytes, err := encrypt(input.Telefono)
	if err != nil {
		http.Error(w, "Error al cifrar teléfono", http.StatusInternalServerError)
		return
	}
	celBytes, err := encrypt(input.Celular)
	if err != nil {
		http.Error(w, "Error al cifrar celular", http.StatusInternalServerError)
		return
	}
	dirBytes, err := encrypt(input.Direccion)
	if err != nil {
		http.Error(w, "Error al cifrar dirección", http.StatusInternalServerError)
		return
	}
	ciuBytes, err := encrypt(input.Ciudad)
	if err != nil {
		http.Error(w, "Error al cifrar ciudad", http.StatusInternalServerError)
		return
	}
	provBytes, err := encrypt(input.Provincia)
	if err != nil {
		http.Error(w, "Error al cifrar provincia", http.StatusInternalServerError)
		return
	}
	fechaBytes, err := encrypt(input.FechaNacimiento)
	if err != nil {
		http.Error(w, "Error al cifrar fecha de nacimiento", http.StatusInternalServerError)
		return
	}
	genBytes, err := encrypt(input.Genero)
	if err != nil {
		http.Error(w, "Error al cifrar género", http.StatusInternalServerError)
		return
	}
	estadoBytes, err := encrypt(input.EstadoCivil)
	if err != nil {
		http.Error(w, "Error al cifrar estado civil", http.StatusInternalServerError)
		return
	}

	// 4) Ejecutar el UPDATE en lugar del INSERT
	_, err = db.ConnDatos.Exec(context.Background(), `
		UPDATE datos_personales
		   SET telefono         = $1,
		       celular          = $2,
		       direccion        = $3,
		       ciudad           = $4,
		       provincia        = $5,
		       fecha_nacimiento = $6,
		       genero           = $7,
		       estado_civil     = $8
		 WHERE id_usuario = $9
	`,
		telBytes,
		celBytes,
		dirBytes,
		ciuBytes,
		provBytes,
		fechaBytes,
		genBytes,
		estadoBytes,
		input.IDUsuario,
	)
	if err != nil {
		http.Error(w, "Error al actualizar datos personales: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5) Responder
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"mensaje":  "Datos personales actualizados correctamente",
		"politica": politica,
	})
}

// --------------------------
// Handler: Eliminar Datos Personales
// --------------------------

func EliminarDatosPersonales(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("id_usuario")
	if uid == "" {
		http.Error(w, "Falta id_usuario", http.StatusBadRequest)
		return
	}
	idUsuario, err := strconv.Atoi(uid)
	if err != nil {
		http.Error(w, "id_usuario inválido", http.StatusBadRequest)
		return
	}

	if _, err := db.ConnDatos.Exec(context.Background(),
		`DELETE FROM datos_personales WHERE id_usuario = $1`, idUsuario,
	); err != nil {
		http.Error(w, "Error al eliminar datos personales", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"mensaje": "Datos personales eliminados correctamente",
	})
}
