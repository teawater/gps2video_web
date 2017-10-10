package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/strava/go.strava"
)

const web_logout = "logout"
const web_activity = "activity"
const web_photos = "photos"
const web_makevideo = "makevideo"
const web_video = "v.mp4"
const activity_layout = "2006-01-02 15:04:05"
const photo_layout = "20060102150405"

func httpInit() {
	makevideoOptionsInit()

	//Setup baseURL
	if serverConf.SSL {
		baseURL = "https://"
	} else {
		baseURL = "http://"
	}
	baseURL += fmt.Sprintf("%s:%d",
		serverConf.DomainName,
		serverConf.Port)
	callbackURL := baseURL + "/exchange_token"
	baseURL += serverConf.DomainDir
	log.Println(baseURL)

	strava.ClientId = serverConf.ClientId
	strava.ClientSecret = serverConf.ClientSecret

	//authenticator setup
	authenticator = &strava.OAuthAuthenticator{
		CallbackURL:            callbackURL,
		RequestClientGenerator: nil,
	}

	path, err := authenticator.CallbackPath()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc(path, authenticator.HandlerFunc(oAuthSuccess, oAuthFailure))
	http.HandleFunc(serverConf.DomainDir, indexHandler)
	http.HandleFunc(serverConf.DomainDir+web_logout, logoutHandler)
	http.HandleFunc(serverConf.DomainDir+web_photos, photosHandler)
	http.HandleFunc(serverConf.DomainDir+web_makevideo, makevideoHandler)
	http.HandleFunc(serverConf.DomainDir+web_video, videoHandler)
}

func formGetOne(r *http.Request, id string) string {
	vals, ok := r.Form[id]
	if (!ok) || (len(vals) < 1) {
		return ""
	}

	return vals[0]
}

func checkCookie(r *http.Request) (uid uint64, token string, err error) {
	var cookie *http.Cookie

	if cookie, err = r.Cookie("uid"); err != nil {
		return
	}
	if uid, err = strconv.ParseUint(cookie.Value, 10, 64); err != nil {
		return
	}

	if cookie, err = r.Cookie("token"); err != nil {
		return
	}
	token = cookie.Value

	if !users.Check(uid, token) {
		err = errors.New(fmt.Sprintf("uid %d token %s is not right",
			uid, token))
		return
	}

	return
}

func addCookie(w http.ResponseWriter, token string) (err error) {
	uid, err := users.Find_add(token)
	if err != nil {
		return
	}

	cookie := http.Cookie{Name: "uid", Value: fmt.Sprintf("%d", uid), Path: serverConf.DomainDir, MaxAge: 86400}
	http.SetCookie(w, &cookie)
	cookie = http.Cookie{Name: "token", Value: token, Path: serverConf.DomainDir, MaxAge: 86400}
	http.SetCookie(w, &cookie)

	return
}

func deleteCookie(w http.ResponseWriter) {
	cookie := http.Cookie{Name: "uid", Path: serverConf.DomainDir, MaxAge: -1}
	http.SetCookie(w, &cookie)
	cookie = http.Cookie{Name: "token", Path: serverConf.DomainDir, MaxAge: -1}
	http.SetCookie(w, &cookie)
}

func httpHead(w http.ResponseWriter) {
	fmt.Fprintf(w, "<html><head><title>GPS2Video</title></head><body>")
}

func httpTail(w http.ResponseWriter) {
	fmt.Fprintf(w, "</body></html>")
}

func httpShowError(w http.ResponseWriter, err string) {
	httpHead(w)
	fmt.Fprintln(w, err, `<br>`)
	fmt.Fprintln(w, `<a href="javascript:history.go(-1)" class="header-back jsBack">返回</a>`)
	httpTail(w)
}

func httpReturnHome(w http.ResponseWriter, str string) {
	httpHead(w)
	fmt.Fprintln(w, str+"<br>")
	fmt.Fprintf(w, `<a href="%s">返回首页</a><br>`, baseURL)
	httpTail(w)
}

func httpCookieError(w http.ResponseWriter) {
	deleteCookie(w)
	httpReturnHome(w, "登录信息有错")
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	deleteCookie(w)
	httpHead(w)
	fmt.Fprintf(w, "已经退出登录")
	httpTail(w)
}

func oAuthSuccess(auth *strava.AuthorizationResponse, w http.ResponseWriter, r *http.Request) {
	addCookie(w, auth.AccessToken)
	//http.Redirect(w, r, baseURL, 301)
	httpReturnHome(w, "登陆成功")
}

func oAuthFailure(err error, w http.ResponseWriter, r *http.Request) {
	httpHead(w)
	if err == strava.OAuthAuthorizationDeniedErr {
		fmt.Fprint(w, "The user clicked the 'Do not Authorize' button on the previous page.\n")
		fmt.Fprint(w, "This is the main error your application should handle.")
	} else if err == strava.OAuthInvalidCredentialsErr {
		fmt.Fprint(w, "You provided an incorrect client_id or client_secret.\nDid you remember to set them at the begininng of this file?")
	} else if err == strava.OAuthInvalidCodeErr {
		fmt.Fprint(w, "The temporary token was not recognized, this shouldn't happen normally")
	} else if err == strava.OAuthServerErr {
		fmt.Fprint(w, "There was some sort of server error, try again to see if the problem continues")
	} else {
		fmt.Fprint(w, err)
	}
	httpTail(w)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// agent := strings.ToLower(r.UserAgent())
	// if strings.Contains(agent, "micromessenger") {
	// fmt.Fprintf(w, "微信显示有问题，请点击左上角，然后点在浏览器打开。")
	// return
	// }

	uid, _, err := checkCookie(r)
	if err != nil {
		deleteCookie(w)

		//need login
		httpHead(w)
		fmt.Fprintf(w, `<a href="%s">访问Strava登陆</a><br>`, authenticator.AuthorizationURL("state1", strava.Permissions.Public, true))
		httpTail(w)
		return
	}

	httpHead(w)
	fmt.Fprintf(w, `<a href="%s">退出登录</a><br><br>`, serverConf.DomainDir+web_logout)
	fmt.Fprintf(w, `<a href="%s">图片管理</a><br><br>`, serverConf.DomainDir+web_photos)

	user_dir := filepath.Join(users.dir, fmt.Sprintf("%d", uid))

	output_dir := filepath.Join(user_dir, "output")
	exist, err := fileIsExist(output_dir)
	if err != nil {
		log.Println(uid, "makevideoHandler fileIsExist:", output_dir, err)
		w.WriteHeader(403)
		return
	}
	if exist {
		fmt.Fprintf(w, `一个视频正在生成中<br><br>`)
	} else {
		fmt.Fprintf(w, `<a href="%s">生成视频</a><br><br>`, serverConf.DomainDir+web_makevideo)
	}

	error_dir := filepath.Join(user_dir, "error")
	exist, err = fileIsExist(error_dir)
	if err != nil {
		log.Println(uid, "makevideoHandler fileIsExist:", error_dir, err)
		w.WriteHeader(403)
		return
	}
	if exist {
		fmt.Fprintf(w, `很遗憾，之前的视频生成出错了。<br><br>`)
	}

	video_dir := filepath.Join(user_dir, "v.mp4")
	exist, err = fileIsExist(video_dir)
	if err != nil {
		log.Println(uid, "makevideoHandler fileIsExist:", video_dir, err)
		w.WriteHeader(403)
		return
	}
	if exist {
		fmt.Fprintf(w, `<a href="%s">视频下载</a><br><br>`, serverConf.DomainDir+web_video)
	}

	fmt.Fprintln(w, `<a href="https://github.com/teawater/gps2video_web">关于</a>`)
	httpTail(w)
}

func photosHandler(w http.ResponseWriter, r *http.Request) {
	uid, _, err := checkCookie(r)
	if err != nil {
		httpCookieError(w)
		return
	}

	photos_dir := filepath.Join(users.dir, fmt.Sprintf("%d", uid), "photos")
	if dir_check_creat(photos_dir, true) != nil {
		log.Println(uid, "photosHandler dir_check_creat:", err)
		w.WriteHeader(403)
		return
	}

	r.ParseForm()
	if r.Method == "POST" {
		_, ok := r.Form["up"]
		if ok {
			reader, err := r.MultipartReader()
			if err != nil {
				log.Println(uid, "photosHandler MultipartReader:", err)
				w.WriteHeader(403)
				return
			}
			filename_tail := 0
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				_, ok := part.Header["Content-Type"]
				if !ok {
					continue
				}
				if len(part.Header["Content-Type"]) < 1 {
					continue
				}
				ctype := part.Header["Content-Type"][0]
				if ctype != "image/jpeg" || part.FormName() != "file" || part.FileName() == "" {
					continue
				}

				//Get filename
				var filename string
				for {
					filename = filepath.Join(photos_dir, fmt.Sprintf("%s%d.jpg", time.Now().Format(photo_layout), filename_tail))
					filename_tail++
					exist, err := fileIsExist(filename)
					if err != nil {
						log.Println(uid, "photosHandler fileIsExist:", filename, err)
						w.WriteHeader(403)
						return
					}
					if !exist {
						break
					}
				}

				dst, err := os.Create(filename)
				if err != nil {
					log.Println(uid, "photosHandler os.Create:", filename, err)
					w.WriteHeader(403)
					return
				}
				if _, err := io.Copy(dst, part); err != nil {
					log.Println(uid, "photosHandler io.Copy:", filename, err)
					w.WriteHeader(403)

					return
				}
				dst.Close()
			}
		}

		_, ok = r.Form["del"]
		if ok {
			for index, val := range r.Form {
				if len(val) != 1 || val[0] != "on" {
					continue
				}
				match, err := filepath.Match(`[0-9]*`, index)
				if err != nil || !match {
					continue
				}
				filename := filepath.Join(photos_dir, index+".jpg")
				exist, err := fileIsExist(filename)
				if err != nil {
					log.Println(uid, "photosHandler fileIsExist:", filename, err)
					w.WriteHeader(403)
					return
				}
				if !exist {
					continue
				}
				err = os.Remove(filename)
				if err != nil {
					log.Println(uid, "photosHandler fileIsExist:", filename, err)
					w.WriteHeader(403)
					return
				}
			}
		}
	} else {
		_, ok := r.Form["show"]
		if ok {
			id := formGetOne(r, "id")
			if id == "" {
				w.WriteHeader(403)
				return
			}

			filename := filepath.Join(photos_dir, id+".jpg")
			exist, err := fileIsExist(filename)
			if err != nil {
				log.Println(uid, "photosHandler fileIsExist:", filename, err)
				w.WriteHeader(403)
				return
			}
			if !exist {
				return
			}
			http.ServeFile(w, r, filename)
			return
		}
	}

	httpHead(w)
	show := `<a href="` + serverConf.DomainDir + `">返回</a><hr>`
	fmt.Fprintln(w, show)
	checkbox := ""
	err = filepath.Walk(photos_dir, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		filename := f.Name()
		match, err := filepath.Match(`[0-9]*.jpg`, filename)
		if err == nil && match {
			fileid := string([]byte(filename)[0 : len(filename)-4])
			checkbox += `<input type="checkbox" name="` + fileid + `">`
			checkbox += `<a href="` + serverConf.DomainDir + web_photos + `?show=1&id=` + fileid + `">` + filename + `</a><br>`
		}
		return nil
	})
	if err != nil {
		log.Println(uid, "filepath.Walk", err)
		w.WriteHeader(403)
		return
	}
	if checkbox != "" {
		show := `<form action="`
		show += serverConf.DomainDir + web_photos
		show += `?del=1" id="del_form" method="post">`
		show += checkbox
		show += `<a href="javascript:
		var nodes = document.getElementById('del_form').childNodes;
		for (index in nodes)
		{
		if (nodes[index].type == 'checkbox')
			nodes[index].checked = true;
		}">Select all</a> `
		show += `<input type="reset" value="Reset" /> <input type="submit" value="Remove" /><br></form><hr>`
		fmt.Fprintln(w, show)
	}

	show = `<a href="javascript:
	var up_form = document.getElementById('up_form');
	var newInput = document.createElement('input');
	newInput.type='file';
	newInput.name='file';
	newInput.accept='image/jpeg';
	up_form.appendChild(newInput);
	up_form.appendChild(document.createElement('br'));">Add</a>`
	show += `<form action="`
	show += serverConf.DomainDir + web_photos
	show += `?up=1" id="up_form" method="post" enctype="multipart/form-data">
	<input type="submit" value="Submit" /> <input type="reset" value="Reset" /><br>
	<input type="file" name="file" id="file" accept="image/jpeg"/><br>
	</form>`
	fmt.Fprintln(w, show)
	httpTail(w)
}

func videoHandler(w http.ResponseWriter, r *http.Request) {
	uid, _, err := checkCookie(r)
	if err != nil {
		httpCookieError(w)
		return
	}

	video_dir := filepath.Join(users.dir, fmt.Sprintf("%d", uid), "v.mp4")
	exist, err := fileIsExist(video_dir)
	if err != nil {
		log.Println(uid, "makevideoHandler fileIsExist:", video_dir, err)
		w.WriteHeader(403)
		return
	}
	if !exist {
		httpReturnHome(w, "没有文件")
	}

	http.ServeFile(w, r, video_dir)
	return
}
