package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
  "io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxFileSize = 100 * 1024 * 1024 // 100MB

type Response struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	ImageURL     string `json:"imageUrl"`
	RawURL       string `json:"rawUrl"`
	ShortenedURL string `json:"shortendUrl"` // Corrected to match the API response
	DeletionURL  string `json:"deletionUrl"`
}

func displayHelp() {
	fmt.Println("Usage: e-z [OPTIONS]")
	fmt.Println("A simple client to interact with the e-z.host API")
	fmt.Println("\nOptions:")
	fmt.Println("  --help, -h                     Display this help message")
	fmt.Println("  --api-key, -a [API_KEY]        Store an API key (prompt if API_KEY is not provided)")
	fmt.Println("  --upload, -u [FILE_PATH]       Upload a file to the API (prompt if FILE_PATH is not provided)")
	fmt.Println("  --upload-raw, -ur [FILE_PATH]  Same as the above option, but copies the raw URL to the clipboard")
	fmt.Println("  --shorten, -s [URL]            Shorten a given URL using the API")
}

func saveApiKey(apiKey string) {
	configDir := filepath.Join(os.Getenv("HOME"), ".config")
	filePath := filepath.Join(configDir, ".e-z_key")

	if err := os.MkdirAll(configDir, os.ModePerm); err != nil {
		log.Fatalf("Error creating config directory: %v", err)
	}

	if err := os.WriteFile(filePath, []byte(apiKey), 0644); err != nil {
		log.Fatalf("Error writing API key to file: %v", err)
	}
	fmt.Println("API key saved successfully!")
}

func readApiKey() string {
	filePath := filepath.Join(os.Getenv("HOME"), ".config", ".e-z_key")
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Println("No API key found. File does not exist.")
		return ""
	}
	return string(data)
}

func isValidMimeType(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	validMimeTypes := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".mp3":  true,
		".wav":  true,
		".mp4":  true,
		".avi":  true,
		".pdf":  true,
		".zip":  true,
		".json": true,
	}
	return validMimeTypes[ext]
}

func copyToClipboard(text string) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("echo -n \"%s\" | xclip -selection clipboard", text))
	if err := cmd.Run(); err != nil {
		log.Printf("Error copying to clipboard: %v", err)
		fmt.Println("Please copy manually.")
	}
}

func uploadFile(filePath, apiKey string, copyRawURL bool) {
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		log.Println("Error: File does not exist.")
		return
	}

	if fileInfo.Size() > maxFileSize {
		log.Println("Error: File size exceeds 100MB.")
		return
	}

	if !isValidMimeType(filePath) {
		log.Println("Error: Invalid MIME type for the file.")
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		log.Printf("Error creating form file: %v", err)
		return
	}

	if _, err := io.Copy(part, file); err != nil {
		log.Printf("Error copying file data: %v", err)
		return
	}

	if err := writer.Close(); err != nil {
		log.Printf("Error closing writer: %v", err)
		return
	}

	req, err := http.NewRequest("POST", "https://api.e-z.host/files", body)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error performing request: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Error: Received non-OK response: %d\nResponse: %s", resp.StatusCode, string(responseBody))
		return
	}

	var jsonResponse Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResponse); err != nil {
		log.Println("Failed to parse JSON response.")
		return
	}

	if jsonResponse.Success {
		urlToCopy := jsonResponse.RawURL
		if !copyRawURL {
			urlToCopy = jsonResponse.ImageURL
		}

		copyToClipboard(urlToCopy)
		fmt.Println("File uploaded and URL copied to clipboard.")
	} else {
		log.Println("Upload failed:", jsonResponse.Message)
	}
}

func shortenURL(apiKey, url string) {
	data := map[string]string{"url": url}
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return
	}

	req, err := http.NewRequest("POST", "https://api.e-z.host/shortener", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error performing request: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Error: Received non-OK response: %d\nResponse: %s", resp.StatusCode, string(responseBody))
		return
	}

	var jsonResponse Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResponse); err != nil {
		log.Println("Failed to parse JSON response.")
		return
	}

	if jsonResponse.Success {
		fmt.Println("Shortened URL:", jsonResponse.ShortenedURL)
		copyToClipboard(jsonResponse.ShortenedURL)
	} else {
		log.Println("URL shortening failed:", jsonResponse.Message)
	}
}

func promptForInput(prompt string) string {
	fmt.Print(prompt)
	var input string
	fmt.Scanln(&input)
	return input
}

func getApiKeyOrPrompt() string {
	apiKey := readApiKey()
	if apiKey == "" {
		apiKey = promptForInput("Enter API Key: ")
	}
	return apiKey
}

func main() {
	if len(os.Args) < 2 {
		displayHelp()
		return
	}

	var filePath string
	var urlToShorten string

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--help", "-h":
			displayHelp()
			return
		case "--api-key", "-a":
			if i+1 < len(os.Args) {
				saveApiKey(os.Args[i+1])
				i++
			} else {
				saveApiKey(promptForInput("Enter API Key: "))
			}
		case "--upload", "-u":
			if i+1 < len(os.Args) {
				filePath = os.Args[i+1]
				i++
			} else {
				filePath = promptForInput("Enter file path to upload: ")
			}
			apiKey := getApiKeyOrPrompt()
			uploadFile(filePath, apiKey, false)
		case "--upload-raw", "-ur":
			if i+1 < len(os.Args) {
				filePath = os.Args[i+1]
				i++
			} else {
				filePath = promptForInput("Enter file path to upload: ")
			}
			apiKey := getApiKeyOrPrompt()
			uploadFile(filePath, apiKey, true)
		case "--shorten", "-s":
			if i+1 < len(os.Args) {
				urlToShorten = os.Args[i+1]
				i++
			} else {
				urlToShorten = promptForInput("Enter URL to shorten: ")
			}
			apiKey := getApiKeyOrPrompt()
			shortenURL(apiKey, urlToShorten)
		default:
			log.Println("Unknown option:", arg)
		}
	}
}
