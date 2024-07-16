package motion

import (
	"errors"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Config struct {
	ControlUrl string `yaml:"control_url,omitempty" json:"control_url,omitempty"`
	ListenHost string `yaml:"listen_host,omitempty" json:"listen_host,omitempty"`
	ListenPort int    `yaml:"listen_port,omitempty" json:"listen_port,omitempty"`
	//
	log *log.Logger
}

var (
	cameras map[string]string
)

func (s *Config) Init(log *log.Logger) {
	s.log = log

	if s.ControlUrl == "" {
		s.ControlUrl = "http://localhost:7999"
	}

	if s.ListenHost == "" {
		s.ListenHost = "127.0.0.1"
	}

	if s.ListenPort == 0 {
		s.ListenPort = 18888
	}

}

func (s *Config) CtrlRequest(path string, contentType string) ([]byte, error) {
	log.Print(s.ControlUrl + path)
	res, err := http.Get(s.ControlUrl + path)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, errors.New("Unexpected response from motion webcontrol: " + res.Status)
	}

	if contentType != "" && !strings.HasPrefix(res.Header.Get("Content-Type"), contentType) {
		return nil, errors.New("Unexpected response: " + res.Header.Get("Content-Type"))
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (s *Config) WebcontrolHtmlOutput(value bool) error {
	onoff := "1"
	contentType := "text/plain"
	if value {
		onoff = "2"
		contentType = "text/html"
	}
	_, err := s.CtrlRequest("/0/config/set?webcontrol_interface="+onoff, contentType)
	return err
}

func (s *Config) Action(camera string, action string) (string, error) {
	if err := s.WebcontrolHtmlOutput(false); err != nil {
		return "", err
	}

	action = regexp.MustCompile(`[^a-z]+`).ReplaceAllString(action, "_")

	body, err := s.CtrlRequest("/"+camera+"/action/"+action, "text/plain")
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *Config) ConfigGet(camera string, setting string) (string, error) {
	if err := s.WebcontrolHtmlOutput(false); err != nil {
		return "", err
	}

	setting = regexp.MustCompile(`[^a-z_-]`).ReplaceAllString(setting, "_")

	body, err := s.CtrlRequest("/"+camera+"/config/get?query="+setting, "text/plain")
	if err != nil {
		return "", err
	}

	if matches := regexp.MustCompile(`(?m)^` + setting + `\s*=\s*(.*?)\s*$`).FindSubmatch(body); matches != nil {
		return string(matches[1]), nil
	}
	return "", errors.New("Motion get setting failed: " + setting)
}

func (s *Config) GetCameras() (map[string]string, error) {
	if err := s.WebcontrolHtmlOutput(true); err != nil {
		return nil, err
	}

	body, err := s.CtrlRequest("/", "text/html")
	if err != nil {
		return nil, err
	}

	rea := regexp.MustCompile(`(?m)^<a href='/(\d+)/'>(.+?)</a>`)

	cameras = make(map[string]string)
	for i, m := range rea.FindAllStringSubmatch(string(body), -1) {
		if i > 0 {
			cameras[m[1]] = m[2]
		}
	}

	return cameras, nil
}

func (s *Config) LastPhoto(camera string) (string, error) {

	target_dir, err := s.ConfigGet(camera, "target_dir")
	if err != nil {
		return "", err
	}

	glob := target_dir + "/**/*.jpg"
	matches, err := filepath.Glob(glob)
	if err != nil {
		return "", err
	}

	if len(matches) > 0 {
		sort.Strings(matches)
		imgFile := matches[len(matches)-1]
		return imgFile, nil
	}
	return "", nil
}
