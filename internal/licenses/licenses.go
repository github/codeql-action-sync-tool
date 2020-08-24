package licenses

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/markbates/pkger"
	"github.com/pkg/errors"
)

const errorMessage = "Error reading licenses."

func PrintLicenses() error {
	fmt.Println("This tool could not have been built without all the wonderful developer communities who maintain these dependencies.")
	err := pkger.Walk("/.licenses", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrap(err, errorMessage)
		}
		if info.IsDir() {
			return nil
		}

		reader, err := pkger.Open(path)
		if err != nil {
			return errors.Wrap(err, errorMessage)
		}
		defer reader.Close()
		content, err := ioutil.ReadAll(reader)
		if err != nil {
			return errors.Wrap(err, errorMessage)
		}
		fmt.Println(string(content))

		return nil
	})
	if err != nil {
		return errors.Wrap(err, errorMessage)
	}
	return nil
}
