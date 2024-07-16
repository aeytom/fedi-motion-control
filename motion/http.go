package motion

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aeytom/fedi-motion-control/photo"
	"github.com/aeytom/fedi-motion-control/toot"
	"github.com/mattn/go-mastodon"
)

var masto *toot.Config

// ListenMotionWebhook …
func (s *Config) ListenMotionWebhook(mcfg *toot.Config) {

	masto = mcfg

	mux := http.NewServeMux()
	mux.Handle("/photo", logHandler(http.HandlerFunc(s.htPhoto)))
	mux.Handle("/notify", logHandler(http.HandlerFunc(s.htNotify)))
	if err := http.ListenAndServe(s.ListenHost+":"+fmt.Sprint(s.ListenPort), mux); err != nil {
		panic(err)
	}
}

// htPhoto handles "File Storage" => "Web Hook URL"
// http://127.0.0.1:18888/photo?file=%f&w=%i&h=%J&x=%K&y=%L&D=%D&n=%n&t=%t
// defined in motioneye "File Storage" field "Web Hook URL"
func (s *Config) htPhoto(w http.ResponseWriter, req *http.Request) {

	cropInfo := photo.CropParam{
		BorderPercent: 20,
	}

	cacheHeader(w)

	file := req.FormValue("file")
	if filepath.Ext(file) != ".jpg" {
		writeResponseError("invalid file extension", errors.New(filepath.Ext(file)), http.StatusBadRequest, w)
		return
	}

	// %t Camera ID number
	camera := "1"
	if val, err := strconv.ParseInt(req.FormValue("t"), 10, 16); err == nil {
		camera = strconv.Itoa(int(val))
	} else {
		writeResponseError("Invalid parameter `t := Camera ID number`", err, http.StatusBadRequest, w)
		return
	}

	if target_dir, err := s.ConfigGet(camera, "target_dir"); err != nil {
		writeResponseError("can not get ´target_dir´", err, http.StatusInternalServerError, w)
		return
	} else if matched, err := path.Match(target_dir+"/*/*.jpg", file); !matched || err != nil {
		writeResponseError("file is not within target_dir "+target_dir, err, http.StatusBadRequest, w)
		return
	}

	cropInfo.File = file

	// http://127.0.0.1:18888/photo?file=%f&w=%i&h=%J&x=%K&y=%L
	if val, err := strconv.ParseInt(req.FormValue("x"), 10, 16); err == nil {
		// %K   X coordinate in pixels of the center point of motion. Origin is upper left corner.
		cropInfo.CenterHorizontal = int(val)
	}
	if val, err := strconv.ParseInt(req.FormValue("y"), 10, 16); err == nil {
		// %L   Y coordinate in pixels of the center point of motion. Origin is upper  left  corner  and
		//      number is positive moving downwards (I may change this soon).
		cropInfo.CenterVertical = int(val)
	}
	if val, err := strconv.ParseInt(req.FormValue("w"), 10, 16); err == nil {
		// %i   Width of the rectangle containing the motion pixels (the rectangle that is shown on  the
		//      image when locate is on).
		cropInfo.Width = int(val)
	}
	if val, err := strconv.ParseInt(req.FormValue("h"), 10, 16); err == nil {
		// %J   Height of the rectangle containing the motion pixels (the rectangle that is shown on the
		//      image when locate is on).
		cropInfo.Height = int(val)
	}

	rtext := fmt.Sprintf("Crop and sending image: '%s' center (%d,%d) size (%d,%d)\n",
		cropInfo.File, cropInfo.CenterHorizontal, cropInfo.CenterVertical, cropInfo.Width, cropInfo.Height)
	s.log.Println(rtext, cropInfo)
	_, _ = io.WriteString(w, rtext)

	if imgFile, err := cropInfo.Crop(); err == nil {
		rtext += fmt.Sprintf("result image: '%s'\n", imgFile)
		log.Println(rtext, cropInfo)
		// defer os.Remove(imgFile)

		err = masto.TootWithImage(mastodon.Toot{
			Status: fmt.Sprintf(
				"motion image: '%s' center (%d,%d) size (%d,%d)",
				cropInfo.File,
				cropInfo.CenterHorizontal,
				cropInfo.CenterVertical,
				cropInfo.Width,
				cropInfo.Height),
			Visibility: mastodon.VisibilityFollowersOnly,
			Language:   "en",
		},
			imgFile)
		if err != nil {
			writeResponseError("post toot", err, http.StatusInternalServerError, w)
			return
		}
	} else {
		writeResponseError("image crop", err, http.StatusInternalServerError, w)
		return
	}

	_, err := io.WriteString(w, rtext)
	if err != nil {
		writeResponseError("response", err, http.StatusInternalServerError, w)
	}
}

func writeResponseError(msg string, err error, code int, w http.ResponseWriter) {
	if err != nil {
		msg += " :: " + err.Error()
	}
	log.Println(msg)
	http.Error(w, msg, code)
}

// htNotify handles http://127.0.0.1:18888/?msg=Bewegung+Event+%v+center(%K,%L)
// defined in motioneye "Motion Notifications" field "Web Hook URL"
func (s *Config) htNotify(w http.ResponseWriter, req *http.Request) {

	var notice []string

	var photoPath = req.FormValue("photo")
	if stat, err := os.Stat(photoPath); err == nil {
		notice = append(notice, "New photo from "+stat.ModTime().Format(time.UnixDate))
		notice = append(notice, "Use /photo to get this photo.")
	}

	if msg := req.FormValue("msg"); msg != "" {
		notice = append(notice, msg)
	}

	cacheHeader(w)
	for _, t := range notice {
		if _, err := io.WriteString(w, t); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	if err := masto.TootWithImage(
		mastodon.Toot{
			Status:     strings.Join(notice, "\n"),
			Visibility: mastodon.VisibilityFollowersOnly,
			Language:   "en",
		}, ""); err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// logHandler …
func logHandler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.String()
		log.Printf("%s \"%s\" %s \"%s\"", r.Method, u, r.Proto, r.UserAgent())
		h.ServeHTTP(w, r)
	}
}

// cacheHeader …
func cacheHeader(w http.ResponseWriter) {
	w.Header().Add("Cache-Control", "must-revalidate, private, max-age=20")
}
