package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
)

type UserData struct {
	LoggedIn bool
	Data     bool //Tells whether a log in attempt has been made
	Success  bool //Tells whether the log in attempt was successful
	Username string
}

type ImgPageData struct {
	LoggedIn  bool
	Username  string
	Found     bool
	Name      string
	SrcName   string
	ExtName   string
	ImagePath string
}

type ImgData struct {
	ImageName string
	ThumbName string
}

type ImgTableData struct {
	LoggedIn   bool
	Username   string
	Images     []ImgData
	Numfound   int
	SearchItem string
}

type ImgTableData2 struct {
	LoggedIn   bool
	Username   string
	Images     [][]ImgData
	Numfound   int
	SearchItem string
}

//Checks cookie data to see is user is logged in.
func (data *UserData) GetLoginData(r *http.Request) {
	session, _ := store.Get(r, "userData")

	if IsNil(session.Values["username"]) { //If cookie not found or user logged out
		data.Data = true
		data.Success = false
		data.LoggedIn = false
	} else {
		data.Data = session.Values["data"].(bool)
		data.Success = session.Values["success"].(bool)
		data.Username = session.Values["username"].(string)
		data.LoggedIn = true
	}
}

//Checks cookie data to see is user is logged in.
func (data *ImgPageData) GetLoginData(r *http.Request) {
	session, _ := store.Get(r, "userData")

	if IsNil(session.Values["username"]) { //If cookie not found or user logged out
		data.LoggedIn = false
	} else {
		data.LoggedIn = true
		data.Username = session.Values["username"].(string)
	}
}

//Checks cookie data to see is user is logged in.
func (data *ImgTableData) GetLoginData(r *http.Request) {
	session, _ := store.Get(r, "userData")

	if IsNil(session.Values["username"]) { //If cookie not found or user logged out
		data.LoggedIn = false
	} else {
		data.Username = session.Values["username"].(string)
		data.LoggedIn = true
	}
}

func (data *ImgTableData2) GetLoginData(r *http.Request) {
	session, _ := store.Get(r, "userData")

	if IsNil(session.Values["username"]) { //If cookie not found or user logged out
		data.LoggedIn = false
	} else {
		data.Username = session.Values["username"].(string)
		data.LoggedIn = true
	}
}

//Helper functions:
//https://mangatmodi.medium.com/go-check-nil-interface-the-right-way-d142776edef1
func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}

type errorData struct {
	ErrString string
	LoggedIn  bool
	Username  string
}

//Generates a new html page displaying the encountered error
//Will display different error pages based in the error variant if given
func DisplayError(w http.ResponseWriter, r *http.Request, err error, errorVariant ...string) {
	if err != nil {
		var tmpl *template.Template

		if len(errorVariant) > 0 {
			switch errorVariant[0] {
			case "file":
				tmpl = template.Must(template.ParseFiles("assets/fileError.html", "assets/Templates.html"))
			}
		} else {
			tmpl = template.Must(template.ParseFiles("assets/error.html", "assets/Templates.html"))
		}

		session, _ := store.Get(r, "userData")
		data := errorData{
			ErrString: err.Error(),
		}

		if IsNil(session.Values["username"]) {
			data.LoggedIn = false
			data.Username = ""
		} else {
			data.LoggedIn = true
			data.Username = session.Values["username"].(string)
		}

		tmpl.Execute(w, data)
	}
}

//Returns all files in the images folder.
//If extension is true, returns with file extensions.
//If extension is false, just returns the file names.
func GetFilenames(extension bool) []string {
	files, err := ioutil.ReadDir("./assets/images")
	ImageNames := make([]string, len(files))

	if err == nil {
		count := 0

		if extension {
			for _, f := range files {
				name := f.Name()

				ImageNames[count] = name
				count++
			}
		} else {
			for _, f := range files {
				name := f.Name()

				//Remove extensions from file names
				name = strings.Replace(name, filepath.Ext(name), "", -1)

				ImageNames[count] = name
				count++
			}
		}
	}

	return ImageNames
}

func FormatName(filename string, newName string) string {
	//Get file extension
	ext := filepath.Ext(filename)
	//Replace extension
	name := strings.Replace(filename, ext, "_"+newName, -1)
	//Add extension back and return
	return name + ".jpg"
}

func GenerateThumnail(imageName string) {
	src, _ := imaging.Open("assets/images/" + imageName)
	thumbPath := "assets/thumbnails/" + FormatName(imageName, "thumb")
	_, err := os.Stat(thumbPath)

	//If the thumbnail does not already exist
	if os.IsNotExist(err) {
		//fmt.Printf("Resizing\n")
		// Resize the cropped image to width = 200px preserving the aspect ratio.
		resized := imaging.Resize(src, 0, 200, imaging.Lanczos)

		imaging.Save(resized, thumbPath)
	}
}

func GenerateAllSizes(imageName string) {
	src, _ := imaging.Open("assets/images/" + imageName)
	srcWidth := src.Bounds().Max.X
	sizes := []int{400, 600, 800, 1000, 1200}

	var path string
	var err error

	//Create a thread group
	var wg sync.WaitGroup
	wg.Add(len(sizes))

	start := time.Now()
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
				defer wg.Done()
				resized := imaging.Resize(src, newSize, 0, imaging.Lanczos)
				imaging.Save(resized, path)
			}(newSize, path)
		}
	}

	//Wait for all threads to finish
	wg.Wait()
	duration := time.Since(start)
	fmt.Printf("Took %v seconds to resize\n", duration)
}

func CreateImageArray() [][]ImgData {
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

	//Format multi-dimensional array
	length := len(images)
	remainder := len(images) % 4

	//Is there a number of images divisible by 4?
	var rows int
	if remainder > 0 {
		rows = (length / 4) + 1
	} else {
		rows = (length / 4)
	}

	imageArray := make([][]ImgData, rows)

	if remainder > 0 {
		for i := 0; i < rows-1; i++ {
			imageArray[i] = make([]ImgData, 4)
		}
		imageArray[rows-1] = make([]ImgData, remainder)

	} else {
		for i := range imageArray {
			imageArray[i] = make([]ImgData, 4)
		}
	}

	count = 0
	for i := 0; i < rows; i++ {
		for j := 0; j < 4; j++ {
			if count >= length {
				break
			}
			imageArray[i][j] = images[count]
			count++
		}
	}

	return imageArray
}

func CreateSearchImageTable(search string) ImgTableData {
	files, _ := ioutil.ReadDir("./assets/images")
	var images []ImgData
	imageData := ImgTableData{
		SearchItem: search,
	}

	//Get all files
	for _, f := range files {
		name := f.Name()
		thumbName := strings.Replace(name, filepath.Ext(name), "_thumb", -1)
		name = strings.Replace(name, filepath.Ext(name), "", -1)

		match := "(?i)" + search
		reg := regexp.MustCompile(match)

		//If name matches, add to array
		if reg.Match([]byte(name)) {
			temp := ImgData{
				ImageName: name,
				ThumbName: thumbName,
			}
			images = append(images, temp)
		}
	}

	imageData.Images = images
	imageData.Numfound = len(images)

	return imageData
}

func GeneratePaginatedTable(pageNum int, skip int) []ImgData {
	filenames := GetFilenames(true)
	skipNum1 := (pageNum - 1) * skip
	skipNum2 := pageNum * skip

	if skipNum2 > len(filenames) {
		skipNum2 = len(filenames)
	}

	filenames = filenames[skipNum1:skipNum2]

	images := make([]ImgData, len(filenames))
	count := 0
	for _, n := range filenames {
		name := n
		thumbName := strings.Replace(name, filepath.Ext(name), "_thumb", -1)
		name = strings.Replace(name, filepath.Ext(name), "", -1)

		images[count].ImageName = name
		images[count].ThumbName = thumbName
		count++
	}

	return images
}

func Log(message string) {
	log, _ := os.OpenFile("./Log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)

	dt := time.Now()

	log.WriteString(dt.Format("2006-01-02 15:04:05") + ": " + message + "\n")
	log.Sync()
	log.Close()
}
