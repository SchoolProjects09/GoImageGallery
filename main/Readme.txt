To use this program, you will need to:
-Go to https://golang.org/dl/ and install go. Afterwards, enter 'go version' into cmd to check that it was installed correctly. 
-To install the library dependencies, run 'go get github.com/gorilla/sessions', 'go get github.com/gorilla/mux', and 'go get github.com/disintegration/imaging' in the folder directory.
-Run 'go build main.go mainLib.go' in the folder directory to compile the exe. Both main.go and mainLib.go need to be in the same directory.
--If you are unnable to build the exe, delete the .mod and .sum files and run the command 'go mod init example.com/main' in the folder directory.
-Run main.exe, then enter http://localhost:3000/gallery into your web browser to view the generated webpage.

*Please note that I have not uploaded the resized images, and they will need to be generated manually by going to /sizes