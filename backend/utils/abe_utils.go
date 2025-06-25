// backend/utils/utils.go
package utils

import (
	"backend/db"
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/fentec-project/gofe/abe"
)

var pubKey *abe.FAMEPubKey
var secKey *abe.FAMESecKey

const (
	pubKeyFile = "abe_public.key"
	secKeyFile = "abe_secret.key"
)

func init() {
	gob.Register(&abe.FAMECipher{})
	gob.Register(&abe.FAMEPubKey{})
	gob.Register(&abe.FAMESecKey{})
}

// Inicializa claves ABE
func InicializarABE() {
	if CargarClavesABE() {
		fmt.Println("Claves ABE cargadas desde archivo.")
		return
	}

	fmt.Println("Generando nuevas claves ABE...")
	scheme := abe.NewFAME()
	var err error
	pubKey, secKey, err = scheme.GenerateMasterKeys()
	if err != nil {
		log.Fatalf("Error al generar claves maestras ABE: %v", err)
	}
	if err := GuardarClavesABE(); err != nil {
		log.Fatalf("Error al guardar claves ABE: %v", err)
	}
	fmt.Println("Claves ABE generadas y guardadas correctamente.")
}

func GuardarClavesABE() error {
	if err := guardarArchivoGob(pubKeyFile, pubKey); err != nil {
		return err
	}
	if err := guardarArchivoGob(secKeyFile, secKey); err != nil {
		return err
	}
	return nil
}

func CargarClavesABE() bool {
	pub, err1 := cargarArchivoGob(pubKeyFile)
	sec, err2 := cargarArchivoGob(secKeyFile)
	if err1 != nil || err2 != nil {
		return false
	}
	pubKey = pub.(*abe.FAMEPubKey)
	secKey = sec.(*abe.FAMESecKey)
	return true
}

func guardarArchivoGob(nombre string, data interface{}) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(data); err != nil {
		return err
	}
	return os.WriteFile(nombre, buf.Bytes(), 0600)
}

func cargarArchivoGob(nombre string) (interface{}, error) {
	data, err := os.ReadFile(nombre)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)

	var result interface{}
	switch nombre {
	case pubKeyFile:
		result = &abe.FAMEPubKey{}
	case secKeyFile:
		result = &abe.FAMESecKey{}
	default:
		return nil, fmt.Errorf("archivo de clave desconocido")
	}

	if err := dec.Decode(result); err != nil {
		return nil, err
	}
	return result, nil
}

// Cifrado ABE
func CifrarDatoABE(dato string, politica string) (*abe.FAMECipher, error) {
	scheme := abe.NewFAME()
	mspStruct, err := abe.BooleanToMSP(politica, false)
	if err != nil {
		return nil, fmt.Errorf("error MSP: %v", err)
	}
	return scheme.Encrypt(dato, mspStruct, pubKey)
}

func DescifrarDatoABEConMaster(cipher *abe.FAMECipher, atributos []string) (string, error) {
	scheme := abe.NewFAME()
	attribKeys, err := scheme.GenerateAttribKeys(atributos, secKey)
	if err != nil {
		return "", fmt.Errorf("error generando claves de atributos: %v", err)
	}
	texto, err := scheme.Decrypt(cipher, attribKeys, pubKey)
	if err != nil {
		return "", fmt.Errorf("error descifrando: %v", err)
	}
	return texto, nil
}

func DescifrarDatoABEConClaveUsuario(cipher *abe.FAMECipher, idUsuario int) (string, error) {
	scheme := abe.NewFAME()

	// 1) Cargar atributos []string del usuario
	atributos, err := CargarAtributosUsuario(idUsuario)
	if err != nil {
		return "", fmt.Errorf("error cargando atributos de usuario: %v", err)
	}

	// 2) Generar attribKeys
	attribKeys, err := scheme.GenerateAttribKeys(atributos, secKey)
	if err != nil {
		return "", fmt.Errorf("error generando claves de atributos: %v", err)
	}

	// 3) Decrypt
	texto, err := scheme.Decrypt(cipher, attribKeys, pubKey)
	if err != nil {
		return "", fmt.Errorf("error descifrando con clave usuario: %v", err)
	}
	return texto, nil
}

func SerializarCipher(cipher *abe.FAMECipher) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(cipher)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DeserializarCipher(data []byte) (*abe.FAMECipher, error) {
	var cipher abe.FAMECipher
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(&cipher)
	if err != nil {
		return nil, err
	}
	return &cipher, nil
}

func ParsePoliticaToAtributos(politica string) []string {
	limpia := strings.ReplaceAll(politica, "(", "")
	limpia = strings.ReplaceAll(limpia, ")", "")
	partes := strings.Split(limpia, " OR ")
	for i := range partes {
		partes[i] = strings.TrimSpace(partes[i])
	}
	return partes
}

// VerificarAccesoDinamico
func VerificarAccesoDinamico(idTercero, idTitular int) bool {
	var atributosRaw []byte
	err := db.Pool.QueryRow(context.Background(), `
		SELECT atributos FROM atributos_terceros
		WHERE id_usuario = $1
		ORDER BY fecha_asignacion DESC LIMIT 1
	`, idTercero).Scan(&atributosRaw)
	if err != nil {
		fmt.Println("Error al obtener atributos:", err)
		return false
	}

	var atributos []string
	if err := json.Unmarshal(atributosRaw, &atributos); err != nil {
		fmt.Println("Error al parsear atributos:", err)
		return false
	}

	rows, err := db.Pool.Query(context.Background(), `
		SELECT p.titulo
		FROM consentimientos c
		JOIN politicas_privacidad p ON c.id_politica = p.id_politica
		WHERE c.id_usuario = $1 AND c.estado = 'activo' AND c.fecha_expiracion > NOW()
	`, idTitular)
	if err != nil {
		fmt.Println("Error al consultar políticas:", err)
		return false
	}
	defer rows.Close()

	var politicas []string
	for rows.Next() {
		var titulo string
		if err := rows.Scan(&titulo); err == nil {
			politicas = append(politicas, titulo)
		}
	}

	for _, atributo := range atributos {
		for _, politica := range politicas {
			if atributo == politica {
				return true
			}
		}
	}
	return false
}

// GuardarAtributosUsuario → guarda solo []string
func GuardarAtributosUsuario(idUsuario int, atributos []string) error {
	fmt.Printf(">>> Guardando atributos en clave ABE (id_usuario=%d): %+v\n", idUsuario, atributos)

	attrBytes, err := json.Marshal(atributos)
	if err != nil {
		fmt.Printf(">>> Error serializando atributos: %v\n", err)
		return fmt.Errorf("error serializando atributos: %v", err)
	}

	_, err = db.Pool.Exec(context.Background(), `
        INSERT INTO claves_abe_usuario (id_usuario, clave, fecha_generacion)
        VALUES ($1, $2, NOW())
        ON CONFLICT (id_usuario) DO UPDATE
        SET clave = EXCLUDED.clave,
            fecha_generacion = EXCLUDED.fecha_generacion,
            version = claves_abe_usuario.version + 1
    `, idUsuario, attrBytes)

	if err != nil {
		fmt.Printf(">>> Error al hacer INSERT/UPDATE en claves_abe_usuario: %v\n", err)
		return fmt.Errorf("error guardando atributos en clave ABE: %v", err)
	}

	fmt.Println(">>> Guardado en claves_abe_usuario exitoso.")
	return nil
}

// CargarAtributosUsuario → devuelve []string
func CargarAtributosUsuario(idUsuario int) ([]string, error) {
	var attrRaw []byte
	err := db.Pool.QueryRow(context.Background(), `
		SELECT clave FROM claves_abe_usuario
		WHERE id_usuario = $1
	`, idUsuario).Scan(&attrRaw)

	if err != nil {
		return nil, fmt.Errorf("error cargando clave ABE usuario: %v", err)
	}

	var atributos []string
	if err := json.Unmarshal(attrRaw, &atributos); err != nil {
		return nil, fmt.Errorf("error deserializando atributos: %v", err)
	}

	return atributos, nil
}

// GetSecKey permite acceso a la secretKey global
func GetSecKey() *abe.FAMESecKey {
	return secKey
}
