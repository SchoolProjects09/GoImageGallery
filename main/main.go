package main

import (
	"errors"
	"fmt"
	"html/template"
	"image"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

var (
	key        = []byte("super-secret-key")   //Designate key value for encrypting cookie data
	store      = sessions.NewCookieStore(key) //Create a global cookie store using the key
	extensions = []string{".jpg", ".JPG", ".png", ".PNG"}
)

//Functions for handling different pages:

//Generates login page and accepts the form data from the page.
//Redirects logged in users to the main page.
//If user fails to log in, returns same page with added error message
func getLogin(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("assets/log_in.html", "assets/Templates.html"))
	var data UserData
	data.GetLoginData(r)

	if data.LoggedIn == false { //If login information not found

		//First request:
		if r.Method != http.MethodPost {
			dataDefault := UserData{
				Data:     false,
				Success:  false,
				Username: "",
			}
			DisplayError(w, r, tmpl.Execute(w, dataDefault))
			return
		}

		session, _ := store.Get(r, "userData")

		//If username and password were entered:
		if r.FormValue("username") != "" && r.FormValue("password") != "" {
			formData := UserData{
				Data:     true,
				Success:  true,
				Username: r.FormValue("username"),
			}
			//Load logged in page
			session.Values["data"] = formData.Data
			session.Values["success"] = formData.Success
			session.Values["username"] = formData.Username
			session.Save(r, w)

			http.Redirect(w, r, "/gallery", http.StatusSeeOther)
			//displayError(w, r, tmpl.Execute(w, formData))
			return
		}

		//If username and password were not both entered:
		formData := UserData{
			Data:    true,
			Success: false,
		}

		tmpl.Execute(w, formData)
	} else { //User is logged in
		http.Redirect(w, r, "/gallery", http.StatusSeeOther)
	}
}

//Generates logout page and sets userData cookie values to nil
func getLogout(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("assets/log_out.html", "assets/Templates.html"))
	session, _ := store.Get(r, "userData")

	session.Values["data"] = true
	session.Values["success"] = false
	session.Values["username"] = nil
	session.Save(r, w)

	DisplayError(w, r, tmpl.Execute(w, nil))
}

//Generates main page, Dynamically generates html based on the number of files in the images folder
func getGallery(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("assets/gallery.html", "assets/Templates.html"))
	var imageData ImgTableData
	imageData.GetLoginData(r)

	files, _ := ioutil.ReadDir("./assets/images")
	images := make([]ImgData, len(files))

	//Get all files
	count := 0
	for _, f := range files {
		name := f.Name()
		thumbName := strings.Replace(name, filepath.Ext(name), "_thumb", -1)
		name = strings.Replace(name, filepath.Ext(name), "", -1)

		images[count].ImageName = name
		images[count].ThumbName = thumbName
		count++
	}
	imageData.Images = images

	DisplayError(w, r, tmpl.Execute(w, imageData))
}

//Generates a /image page matching the entered file name
//Generates an error page if the file is not found
func getImage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("assets/image.html", "assets/Templates.html"))
	var imgData ImgPageData
	imgData.GetLoginData(r)

	vars := mux.Vars(r)
	imgName := vars["imgName"]
	var imgPath string
	var err error
	var ext string

	for _, ex := range extensions {
		//Try all file extensions to find the correct one
		imgPath = "assets/images/" + imgName + ex

		//Stat needs path without /, while the html parser needs it with a /
		//Stat checks if the file exists
		_, err = os.Stat(imgPath)

		if !os.IsNotExist(err) {
			//If file extension matches, break out of loop
			ext = ex
			break
		}
	}

	//If the file was matched to a file extension, display the found file
	if !os.IsNotExist(err) {
		imgData.ExtName = (imgName + ext)
		imgData.Name = imgName
		//prevent issues due to spaces in the image name
		imgData.SrcName = strings.Replace(imgName, " ", "%20", -1)
		imgData.ImagePath = ("/" + imgPath) //Add / to path for parser
		imgData.Found = true
	} else {
		//Image is not found, display error
		imgData.Found = false
	}

	DisplayError(w, r, tmpl.Execute(w, imgData))
}

func getUpload(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("assets/upload.html", "assets/Templates.html"))

	var data UserData
	data.GetLoginData(r)

	DisplayError(w, r, tmpl.Execute(w, data))
}

//Code from https://freshman.tech/file-upload-golang/
const sizeMulti = 10 // 10MB
const maxUploadSize = 1024 * 1024 * sizeMulti

// Progress is used to track the progress of a file upload.
// It implements the io.Writer interface so it can be passed
// to an io.TeeReader()
type Progress struct {
	TotalSize int64
	BytesRead int64
}

// Write is used to satisfy the io.Writer interface.
// Instead of writing somewhere, it simply aggregates
// the total bytes on each read
func (pr *Progress) Write(p []byte) (n int, err error) {
	n, err = len(p), nil
	pr.BytesRead += int64(n)
	pr.Print()
	return
}

// Print displays the current progress of the file upload
func (pr *Progress) Print() {
	if pr.BytesRead == pr.TotalSize {
		fmt.Println("DONE!")
		return
	}

	fmt.Printf("File upload in progress: %d\n", pr.BytesRead)
}

//Accepts upload requests and checks if they are valid.
//Valid requests will result in the files being saved to the
//images folder with the same name and extension
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	currentFileNames := GetFilenames(true)

	if r.Method != "POST" {
		DisplayError(w, r, errors.New(fmt.Sprintf("Cannot upload: %s method not allowed", r.Method)))
		return
	}

	// 32 MB is the default used by FormFile
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		DisplayError(w, r, errors.New("Form data cannot be read"), "file")
		return
	}

	// get a reference to the fileHeaders
	files := r.MultipartForm.File["fileInput"]

	if len(files) < 1 {
		DisplayError(w, r, errors.New(fmt.Sprintf("Cannot upload: No file has been submitted")), "file")
		return
	}

	fileHeader := files[0]

	for _, fileName := range currentFileNames {
		//Check if the file already exists
		if fileHeader.Filename == fileName {
			DisplayError(w, r, errors.New(fmt.Sprintf("Cannot upload: The file %s already exists", fileHeader.Filename)), "file")
			return
		}
	}

	if fileHeader.Size > maxUploadSize {
		DisplayError(w, r, errors.New(fmt.Sprintf("Cannot upload: The file %s is too big. The maximum file size is %vMB in size", fileHeader.Filename, sizeMulti)), "file")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		DisplayError(w, r, err, "file")
		return
	}

	defer file.Close()

	buff := make([]byte, 512)
	_, err = file.Read(buff)
	if err != nil {
		DisplayError(w, r, err, "file")
		return
	}

	filetype := http.DetectContentType(buff)
	if filetype != "image/jpeg" && filetype != "image/png" {
		DisplayError(w, r, errors.New(fmt.Sprintf("The provided file format (%s) is not allowed. Please upload a jpeg or png image", filepath.Ext(fileHeader.Filename))), "file")
		return
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		DisplayError(w, r, err, "file")
		return
	}

	err = os.MkdirAll("./assets/images", os.ModePerm)
	if err != nil {
		DisplayError(w, r, err, "file")
		return
	}

	//Save file as filename.extension
	//ext := filepath.Ext(fileHeader.Filename)
	f, err := os.Create(fmt.Sprintf("./assets/images/%s", fileHeader.Filename))
	if err != nil {
		DisplayError(w, r, err, "file")
		return
	}

	defer f.Close()

	pr := &Progress{
		TotalSize: fileHeader.Size,
	}

	_, err = io.Copy(f, io.TeeReader(file, pr))
	if err != nil {
		DisplayError(w, r, err, "file")
		return
	}

	//GenerateAllSizes(fileHeader.Filename)
	imageName := fileHeader.Filename
	src, _ := imaging.Open("assets/images/" + imageName)
	srcWidth := src.Bounds().Max.X
	sizes := []int{400, 600, 800, 1000, 1200}

	var path string

	//Create a thread group
	//var wg sync.WaitGroup
	//wg.Add(len(sizes))

	//start := time.Now()
	//For every size, create a new image if the size is less than the original image size
	for _, size := range sizes {
		var newSize int
		if srcWidth <= size {
			newSize = srcWidth
		} else {
			newSize = size
		}

		path = "assets/resized/" + FormatName(imageName, fmt.Sprintf("%v", size))
		_, err = os.Stat(path)

		//If the thumbnail does not already exist
		if os.IsNotExist(err) {
			//Run this function on a new thread
			go func(newSize int, path string) {
				//defer wg.Done()
				resized := imaging.Resize(src, newSize, 0, imaging.Lanczos)
				fmt.Printf("%v Finished\n", newSize)
				imaging.Save(resized, path)
			}(newSize, path)
		}
	}
	GenerateThumnail(fileHeader.Filename)

	Log(fileHeader.Filename + " uploaded")

	var data UserData
	data.GetLoginData(r)

	tmpl := template.Must(template.ParseFiles("assets/uploaded.html", "assets/Templates.html"))
	DisplayError(w, r, tmpl.Execute(w, data))
	return
	//wg.Wait()
	//duration := time.Since(start)
	//fmt.Printf("Took %v seconds to resize", duration)
}

//Redirects search requests to the search handler, which is search/{search string}
func searchRedirect(w http.ResponseWriter, r *http.Request) {
	search := r.FormValue("search")
	searchUrl := "search/" + search

	http.Redirect(w, r, searchUrl, http.StatusSeeOther)
}

//Handles search requests.
//When a request is made, you are redirected to a search page
//and this handler gets the search string from the url.
//It then uses regex in order to search the files and find matches.
func searchHandler(w http.ResponseWriter, r *http.Request) {
	//Get variables
	tmpl := template.Must(template.ParseFiles("assets/search.html", "assets/Templates.html"))

	//Get search string from url
	vars := mux.Vars(r)
	search := vars["search"]

	imageData := CreateSearchImageTable(search)
	imageData.GetLoginData(r)

	DisplayError(w, r, tmpl.Execute(w, imageData))
}

func removalHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileName := vars["file"]
	thumbName := FormatName(fileName, "thumb")
	os.Remove("./assets/thumbnails/" + thumbName)

	sizes := []int{400, 600, 800, 1000, 1200}
	var path string
	//For every size, create a new image if the size is less than the original image size
	for _, size := range sizes {
		path = FormatName(fileName, fmt.Sprintf("%v", size))
		os.Remove("./assets/resized/" + path)
	}

	os.Remove("./assets/images/" + fileName)

	Log(fileName + " deleted")
	http.Redirect(w, r, "/gallery", http.StatusSeeOther)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	file := vars["file"]

	filepath := "./assets/images/" + file
	//Sends just the file data, not any page data
	http.ServeFile(w, r, filepath)
}

//404 page not found handler
func notFound(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("assets/404.html", "assets/Templates.html"))

	var data UserData
	data.GetLoginData(r)

	DisplayError(w, r, tmpl.Execute(w, data))
}

//Debug/test pages:

//Debugging page that prints all file names
func checkFiles(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Table 1:\n")
	images := GeneratePaginatedTable(1, 5)
	for _, i := range images {
		fmt.Fprintf(w, "%s\n", i.ImageName)
	}
	fmt.Fprintf(w, "\nTable 2:\n")
	images = GeneratePaginatedTable(2, 5)
	for _, i := range images {
		fmt.Fprintf(w, "%s\n", i.ImageName)
	}
	fmt.Fprintf(w, "\nTable 3:\n")
	images = GeneratePaginatedTable(1, 25)
	for _, i := range images {
		fmt.Fprintf(w, "%s\n", i.ImageName)
	}
	fmt.Fprintf(w, "\nTable 4:\n")
	images = GeneratePaginatedTable(2, 8)
	for _, i := range images {
		fmt.Fprintf(w, "%s\n", i.ImageName)
	}
}

func thumbTest1(w http.ResponseWriter, r *http.Request) {
	name := "Jellyfish.jpg"
	GenerateThumnail(name)

	path, _ := os.Open("assets/thumbnails/" + FormatName(name, "thumb"))
	stats, _, _ := image.DecodeConfig(path)

	fmt.Fprintf(w, "Width: %v Height: %v", stats.Width, stats.Height)
	/*
		files := getFilenames(true)
		for _, f := range files {
			fmt.Fprintf(w, "%v\n", generateThumbName(f))
		}
	*/
}

func thumbTest2(w http.ResponseWriter, r *http.Request) {
	files := GetFilenames(true)

	for _, file := range files {
		GenerateThumnail(file)
	}
}

func generateAllImageSizes(w http.ResponseWriter, r *http.Request) {
	files := GetFilenames(true)

	for _, file := range files {
		go GenerateAllSizes(file)
	}
}

type imgTableData2 struct {
	LoggedIn bool
	Username string
	Images   [][]ImgData
}

func formatTest(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("assets/fileFormat.html", "assets/Templates.html"))
	var data1 ImgTableData
	data1.GetLoginData(r)

	imageArray := CreateImageArray()

	data2 := imgTableData2{
		LoggedIn: data1.LoggedIn,
		Username: data1.Username,
		Images:   imageArray,
	}

	tmpl.Execute(w, data2)
}

type rangeData struct {
	Range [4][4]ImgData
}

func rangeTest(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("assets/range.html"))

	var data rangeData
	count := 1

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			data.Range[i][j].ImageName = fmt.Sprintf("%v", count)
			count++
		}
	}

	DisplayError(w, r, tmpl.Execute(w, data))
}

func main() {
	fs := http.FileServer(http.Dir("assets"))             //Define assets folder as file server
	fs2 := http.FileServer(http.Dir("assets/images"))     //Define images folder as file server
	fs3 := http.FileServer(http.Dir("assets/thumbnails")) //Define thumbnails folder as file server
	fs4 := http.FileServer(http.Dir("assets/resized"))    //Define other images folder as file server

	r := mux.NewRouter() //Create router

	//Handle requests for assets
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))
	//Handle requests for images
	r.PathPrefix("/assets/images/").Handler(http.StripPrefix("/assets/images/", fs2))
	//Handle requests for images
	r.PathPrefix("/assets/thumbnails/").Handler(http.StripPrefix("/assets/thumbnails/", fs3))
	//Handle other image requests
	r.PathPrefix("/assets/resized/").Handler(http.StripPrefix("/assets/resized/", fs4))

	r.HandleFunc("/login", getLogin)         //Handle login page
	r.HandleFunc("/gallery", getGallery)     //Handle main gallery page
	r.HandleFunc("/logout", getLogout)       //Handle logout page
	r.HandleFunc("/upload", getUpload)       //Handle upload page
	r.HandleFunc("/uploaded", uploadHandler) //Handle file uploads
	r.HandleFunc("/search", searchRedirect)  //Redirect search requests

	//Handle requests for image files to be displayed
	r.HandleFunc("/image/{imgName}", getImage)
	//Handle search requests
	r.HandleFunc("/search/{search}", searchHandler)
	//Handle deletion requests
	r.HandleFunc("/delete/{file}", removalHandler)
	//Handle download requests
	r.HandleFunc("/download/{file}", downloadHandler)

	r.NotFoundHandler = http.HandlerFunc(notFound)

	//Debug/test pages:
	r.HandleFunc("/files", checkFiles)            //Print all files
	r.HandleFunc("/range", rangeTest)             //Test multidimensional arrays parsing
	r.HandleFunc("/thumb1", thumbTest1)           //Generate single thumbnail
	r.HandleFunc("/thumb2", thumbTest2)           //Generate all thumbnails
	r.HandleFunc("/format", formatTest)           //Test multidimensional array formating
	r.HandleFunc("/sizes", generateAllImageSizes) //Resize all images in folder

	http.ListenAndServe(":3000", r) //Attach router to port
}
