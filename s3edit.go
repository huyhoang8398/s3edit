package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3Edit struct {
	BucketName string
	Key        string
	FileName   string
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: s3edit s3://bucket-name/path/to/file")
	}

	s3Path, err := ParseS3Path(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	pad := S3Edit{
		BucketName: s3Path.BucketName,
		Key:        s3Path.Key,
		FileName:   s3Path.FileName,
	}

	err = pad.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func (s *S3Edit) Run() error {
	sess, err := session.NewSession()
	if err != nil {
		return err
	}

	svc := s3.New(sess)

	resp, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(s.Key),
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var fileContent bytes.Buffer
	_, err = io.Copy(&fileContent, resp.Body)
	if err != nil {
		return err
	}

	tmpfile, err := os.CreateTemp("", "S3Edit-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write(fileContent.Bytes())
	if err != nil {
		return err
	}
	err = tmpfile.Close()
	if err != nil {
		return err
	}

	editor := getUserInput("Enter the text editor to use (vi, vim, nano): ", []string{"vi", "vim", "nano"})
	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	updatedFileContent, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return err
	}

	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(s.BucketName),
		Key:         aws.String(s.Key),
		Body:        bytes.NewReader(updatedFileContent),
		ContentType: aws.String(http.DetectContentType(updatedFileContent)),
	})
	if err != nil {
		return err
	}

	fmt.Println("File saved successfully.")
	return nil
}

func getUserInput(prompt string, allowedValues []string) string {
	for {
		var input string
		fmt.Print(prompt)
		fmt.Scanln(&input)

		trimmedInput := strings.TrimSpace(input)
		for _, allowedValue := range allowedValues {
			if trimmedInput == allowedValue {
				return trimmedInput
			}
		}

		fmt.Printf("Invalid input. Please enter one of %v.\n", allowedValues)
	}
}

func ParseS3Path(s3Path string) (*S3Edit, error) {
	if !strings.HasPrefix(s3Path, "s3://") {
		return nil, fmt.Errorf("Invalid S3 path: %s", s3Path)
	}

	parts := strings.SplitN(s3Path[5:], "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid S3 path: %s", s3Path)
	}

	fileName := parts[1][strings.LastIndex(parts[1], "/")+1:]
	return &S3Edit{
		BucketName: parts[0],
		Key:        parts[1],
		FileName:   fileName,
	}, nil
}
