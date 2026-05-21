package decoders

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

const DEFAULT_RAILBOX_HTTP_ADDRESS = "http://192.168.4.1"
const SOUND_PACKAGE_CLEAR_ENDPOINT = "/delete?p=/%d/all"
const SOUND_PACKAGE_DELETE_FILE_ENDPOINT = "/delete?p=/%d/%s"
const SOUND_PACKAGE_LIST_ENDPOINT = "/?p=/%d/"
const SOUND_PACKAGE_UPLOAD_ENDPOINT = "/upload?p=/%d/%s"
const DEFAULT_TIMEOUT = 10 * time.Second

type Option func(*RailboxRB23xx)

func WithTimeout(seconds uint16) Option {
	return func(d *RailboxRB23xx) {
		d.client.Timeout = time.Duration(seconds) * time.Second
	}
}

type RailboxRB23xx struct {
	client *http.Client
}

func NewRailboxRB23xx(opts ...Option) *RailboxRB23xx {
	d := &RailboxRB23xx{
		client: newHTTPClient(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: DEFAULT_TIMEOUT,
	}
}

func (d *RailboxRB23xx) httpGet(endpoint string) (*http.Response, error) {
	url := DEFAULT_RAILBOX_HTTP_ADDRESS + endpoint
	resp, err := d.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to loco wifi (are you connected to loco wifi? is loco wifi function on?): %w", err)
	}
	return resp, nil
}

func (d *RailboxRB23xx) ClearSoundSlot(slot uint8) error {
	resp, err := d.httpGet(fmt.Sprintf(SOUND_PACKAGE_CLEAR_ENDPOINT, slot))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// reFileEntry matches a file row in the listing HTML, capturing name and size in KB.
// Example row fragment: placeholder='F11_Decouple.wav'> </td><td>file</td><td align='right'>108</td>
var reFileEntry = regexp.MustCompile(`placeholder='([^']+)'[^<]*</td><td>file</td><td[^>]*>(\d+)</td>`)

// RemoteFileInfo holds metadata about a file on the decoder.
type RemoteFileInfo struct {
	Name   string
	SizeKB int64
}

// ListSoundSlot returns the files present in the given slot on the decoder.
func (d *RailboxRB23xx) ListSoundSlot(slot uint8) ([]RemoteFileInfo, error) {
	resp, err := d.httpGet(fmt.Sprintf(SOUND_PACKAGE_LIST_ENDPOINT, slot))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read listing response: %w", err)
	}

	matches := reFileEntry.FindAllSubmatch(body, -1)
	files := make([]RemoteFileInfo, 0, len(matches))
	for _, m := range matches {
		var sizeKB int64
		fmt.Sscan(string(m[2]), &sizeKB)
		files = append(files, RemoteFileInfo{
			Name:   string(m[1]),
			SizeKB: sizeKB,
		})
	}
	return files, nil
}

// DeleteSoundFile deletes a single file from the given slot on the decoder.
func (d *RailboxRB23xx) DeleteSoundFile(slot uint8, filename string) error {
	resp, err := d.httpGet(fmt.Sprintf(SOUND_PACKAGE_DELETE_FILE_ENDPOINT, slot, filename))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete %q failed with HTTP %d", filename, resp.StatusCode)
	}
	return nil
}

// UploadSoundFile uploads a file to the given slot on the decoder.
func (d *RailboxRB23xx) UploadSoundFile(slot uint8, filename string, content io.Reader) error {
	data, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", filename, err)
	}

	url := DEFAULT_RAILBOX_HTTP_ADDRESS + fmt.Sprintf(SOUND_PACKAGE_UPLOAD_ENDPOINT, slot, filename)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to build upload request for %q: %w", filename, err)
	}
	req.Header.Set("Content-Type", "multipart/form-data")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("upload %q failed: %w", filename, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("upload %q failed with HTTP %d", filename, resp.StatusCode)
	}
	return nil
}
