package app

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/kordiseps/media-gallery/model"
)

var allowedDirs []string

type App struct {
	Router *mux.Router
}

func (a *App) Init() {

	var varsConfig model.VarsConfig
	vars, err := os.ReadFile("vars.json")
	// vars, err := os.ReadFile("../../vars.json")
	if err != nil {
		fmt.Println("could not read vars.json :", err)
		return
	}
	err = json.Unmarshal(vars, &varsConfig)
	if err != nil {
		fmt.Println("could not deserialize vars.json :", err)
	}
	allowedDirs = append(allowedDirs, varsConfig.Dirs...)

	a.Router = mux.NewRouter()
	a.Router.HandleFunc("/", home)
	// a.Router.HandleFunc("/dirs", dirs)
	a.Router.HandleFunc("/content", rootContent).Methods("GET", "OPTIONS")
	a.Router.HandleFunc("/content/{dir}", content).Methods("GET", "OPTIONS")
	a.Router.HandleFunc("/file/{dir}", file).Methods("GET", "OPTIONS")
}

func (a *App) Run(addr string) {
	fmt.Println("listening on " + addr)

	// Where ORIGIN_ALLOWED is like `scheme://dns[:port]`, or `*` (insecure)
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "OPTIONS"})

	log.Fatal(http.ListenAndServe(addr, handlers.CORS(originsOk, headersOk, methodsOk)(a.Router)))

}

func jsonResponse(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	// w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func downloadResponse(w http.ResponseWriter, f *os.File) {
	contentType, err := getFileContentType(f)
	if err != nil {
		panic(err)
	}

	f.Seek(0, 0)

	fi, err := f.Stat()
	if err != nil {
		panic(err)
	}

	// w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(f.Name()))
	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(200)

	_, err = io.Copy(w, f)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, 200, `
- '/' : Home endpoint. Returns endpoint details.  
- '/content' : Root directories endpoint. Returns available root directories that specified in 'vars.json'. For content, there is 'actualPath' property.   
- '/content/{dir}' : Content endpoint. Returns directory paths and image file paths, also thumbnail image path. There is 'actualPath' to retrieve actual image content or folder content. This endpoint also has paging functionality. To do that, start index('s') and end index('e') need to be specified as query parameters. Eg. 'http://localhost:8080/files/3?s=3&&e=5'. Otherwise, the endpoint will return from 0 (zero) to limit.
- '/file/{path}' : File endpoint. Returns actual image content as base64.
`)

}

func rootContent(w http.ResponseWriter, r *http.Request) {

	var files []model.File
	for i := range allowedDirs {

		encodedPath := url.QueryEscape(allowedDirs[i])
		files = append(files, model.File{
			Name:       filepath.Base(allowedDirs[i]),
			ActualPath: encodedPath,
			ThumbPath:  "",
			IsDir:      true,
		})
	}

	response := model.FilesResponse{}
	response.Files = files
	jsonResponse(w, 200, response)
}

func content(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	folderPath := vars["dir"]

	if len(folderPath) == 0 {
		var files []model.File
		for i := range allowedDirs {
			files = append(files, model.File{
				Name:       allowedDirs[i],
				ActualPath: allowedDirs[i],
				ThumbPath:  "",
				IsDir:      true,
			})
		}
		response := model.FilesResponse{}
		response.Files = files
		jsonResponse(w, 200, response)
	}

	v := r.URL.Query()

	s := v.Get("s")
	e := v.Get("e")

	intS, err := strconv.Atoi(s)
	if err != nil {
		intS = 0
	}
	intE, err := strconv.Atoi(e)
	if err != nil {
		intE = 5
	}
	response := filesFromDir(folderPath, intS, intE)
	response.Next = folderPath + response.Next
	jsonResponse(w, 200, response)
}

func file(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	path := vars["dir"]

	decodedValue, _ := url.QueryUnescape(path)

	input, err := os.Open(decodedValue)
	if err != nil {
		fmt.Println("read file error:", err)
		jsonResponse(w, 200, `none`)
	}
	defer input.Close()

	downloadResponse(w, input)
}