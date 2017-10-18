package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/teawater/go.strava"
	"github.com/tkrajina/gpxgo/gpx"
)

type BaseOption struct {
	//Used by config.ini
	configName string //If not set, will same with index
	required   bool

	//Used by http
	shortInfo string //If not set, will same with index
	longInfo  string

	//true if need record to usermap
	needRec bool
}

func (this *BaseOption) Init(index string) {
	if this.configName == "" {
		this.configName = index
	}
	if this.shortInfo == "" {
		this.shortInfo = index
	}
}

func (this *BaseOption) Show() bool {
	return true
}

func (this *BaseOption) GetshortInfo() string {
	return this.shortInfo
}

func (this *BaseOption) GetlongInfo() string {
	return this.longInfo
}

func (this *BaseOption) Getrequired() bool {
	return this.required
}

func (this *BaseOption) FormHaveData(form []string) bool {
	if len(form) < 1 {
		return false
	}

	if len(form[0]) == 0 {
		return false
	}

	return true
}

func (this *BaseOption) Form2String(form []string) (val string, err error) {
	if len(form) < 1 {
		err = errors.New("提交数据出错")
		return
	}

	val = form[0]

	return
}

func (this *BaseOption) Form2Config(form []string, uid uint64) (config string, err error) {
	return
}

type Int64Option struct {
	BaseOption
	defaultVal string
	min        int64
	max        int64 //If set to 0, will not check max
}

func (this *Int64Option) GetHtmlInput(service *CurrentAthleteService, index string) (html string, err error) {
	html = `<input type="text" name="` + index + `" value="` + this.defaultVal + `">`
	return
}

func (this *Int64Option) Form2Int64(form []string) (num int64, err error) {
	str, err := this.Form2String(form)
	if err != nil {
		return
	}
	num, err = strconv.ParseInt(str, 10, 64)
	return
}

func (this *Int64Option) Form2Config(form []string, uid uint64) (config string, err error) {
	var num int64
	if num, err = this.Form2Int64(form); err != nil {
		return
	}
	if num < this.min {
		err = fmt.Errorf("设置的值%d小于最小值%d", num, this.min)
		return
	}
	if this.max != 0 && num > this.max {
		err = fmt.Errorf("设置的值%d大于最大值%d", num, this.max)
		return
	}

	config = fmt.Sprintf("%s=%d\n", this.configName, num)
	return
}

type TrackIdOption struct {
	Int64Option
}

func (this *TrackIdOption) GetHtmlInput(service *CurrentAthleteService, index string) (html string, err error) {
	activities, err := service.ListActivities().Do()
	if err != nil {
		err = errors.New("strava出错:" + err.Error())
		return
	}

	html += `<select name="` + index + `">`
	for _, activity := range activities {
		html += `<option value="` + fmt.Sprintf("%d", activity.Id) + `">`
		html += activity.Name + activity.StartDateLocal.Format(activity_layout)
		html += `</option>`
	}
	html += `</select>`
	return
}

type Float64Option struct {
	BaseOption
}

func (this *Float64Option) GetHtmlInput(service *CurrentAthleteService, index string) (html string, err error) {
	html = `<input type="text" name="` + index + `" value="">`
	return
}

func (this *Float64Option) Form2Float64(form []string) (num float64, err error) {
	str, err := this.Form2String(form)
	if err != nil {
		return
	}
	num, err = strconv.ParseFloat(str, 64)
	return
}

type PhotosTimezoneOption struct {
	Float64Option
}

func (this *PhotosTimezoneOption) Float642Config(num float64) (config string, err error) {
	fi, f := math.Modf(num)
	i := int64(fi)

	if i < -12 || i > 13 || (f != 0 && f != 0.5) {
		err = fmt.Errorf("格式不对")
		return
	}

	if f == 0 {
		config = fmt.Sprintf("%s=%d\n", this.configName, i)
	} else {
		config = fmt.Sprintf("%s=%f\n", this.configName, num)
	}

	return
}

func (this *PhotosTimezoneOption) Form2Config(form []string, uid uint64) (config string, err error) {
	var num float64
	if num, err = this.Form2Float64(form); err != nil {
		return
	}

	config, err = this.Float642Config(num)
	return
}

type ListOption struct {
	BaseOption
	defaultVal string
	Val        []string
	Info       []string
}

func (this *ListOption) GetHtmlInput(service *CurrentAthleteService, index string) (html string, err error) {
	for i := range this.Info {
		checked := ""
		if this.Val[i] == this.defaultVal {
			checked = ` checked="checked"`
		}
		html += `<input type="radio" name="` + index + `" value="` + this.Val[i] + `"` + checked + `>`
		html += this.Info[i] + `<br>`
	}
	return
}

type PhotosOption struct {
	ListOption
}

func (this *PhotosOption) Form2Config(form []string, uid uint64) (config string, err error) {
	str, err := this.Form2String(form)
	if err != nil {
		return
	}

	if str == "none" {
		return
	}

	var photos_dir string
	if str == "local" {
		photos_dir = filepath.Join(users.dir, fmt.Sprintf("%d", uid), "photos")
		err = dir_check_creat(photos_dir, true)
		if err != nil {
			log.Println(uid, "PhotosOption Form2Config dir_check_creat:", err)
			return
		}
	} else {
		photos_dir = filepath.Join(users.dir, fmt.Sprintf("%d", uid), "output", "photos")
	}

	config = fmt.Sprintf("%s=%s\n", this.configName, photos_dir)
	return
}

type BoolOption struct {
	BaseOption
	defaultVal bool
}

func (this *BoolOption) FormHaveData(form []string) bool {
	return true
}

func (this *BoolOption) GetHtmlInput(service *CurrentAthleteService, index string) (html string, err error) {
	checked := ""
	if this.defaultVal {
		checked = ` checked="checked"`
	}
	html = fmt.Sprintf(`<input type="checkbox" name="%s" value="%s"%s>`,
		index, index, checked)
	return
}

func (this *BoolOption) Form2Bool(form []string) bool {
	if len(form) < 1 {
		return false
	}
	return true
}

type SendEmailOption struct {
	BoolOption
}

func (this *SendEmailOption) Show() bool {
	return (serverConf.SmtpServer != "")
}

type MakevideoOptioner interface {
	Init(index string)

	Show() bool
	GetshortInfo() string
	GetlongInfo() string
	Getrequired() bool

	GetHtmlInput(service *CurrentAthleteService, index string) (html string, err error)

	FormHaveData(form []string) bool
	Form2Config(form []string, uid uint64) (config string, err error)
}

var makevideoOptions map[string]MakevideoOptioner
var show_index []string
var photosTimezoneOption *PhotosTimezoneOption

func makevideoOptionsInit() {
	makevideoOptions = make(map[string]MakevideoOptioner)
	show_index = make([]string, 0)

	makevideoOptions["trackid"] = &TrackIdOption{
		Int64Option: Int64Option{
			BaseOption: BaseOption{
				shortInfo: "选择要生成视频的轨迹",
				required:  true,
			},
		},
	}
	show_index = append(show_index, "video_width")

	makevideoOptions["video_width"] = &Int64Option{
		BaseOption: BaseOption{
			shortInfo: "视频宽度",
			longInfo:  "google map免费版的限制，最大640。",
			required:  true,
		},
		defaultVal: "640",
		min:        1,
		max:        640,
	}
	show_index = append(show_index, "video_width")

	makevideoOptions["video_height"] = &Int64Option{
		BaseOption: BaseOption{
			shortInfo: "视频高度",
			longInfo:  "google map免费版的限制，最大640。",
			required:  true,
		},
		defaultVal: "640",
		min:        1,
		max:        640,
	}
	show_index = append(show_index, "video_height")

	makevideoOptions["video_border"] = &Int64Option{
		BaseOption: BaseOption{
			shortInfo: "边框宽度",
			longInfo:  "视频中轨迹到边框的距离",
			required:  true,
		},
		defaultVal: "10",
		min:        1,
		max:        640,
	}
	show_index = append(show_index, "video_border")

	makevideoOptions["video_limit_secs"] = &Int64Option{
		BaseOption: BaseOption{
			shortInfo: "生成视频的最大秒数",
			longInfo:  "程序将自动设置选项video_fps, speed, photos_show_secs 和 trackinfo_show_sec。友情提醒：微信朋友圈视频限制时间为10秒。",
		},
		min: 3,
	}
	show_index = append(show_index, "video_limit_secs")

	makevideoOptions["photos_dir"] = &PhotosOption{
		ListOption: ListOption{
			BaseOption: BaseOption{
				shortInfo: "在视频中增加照片",
				longInfo:  `视频中插入照片，软件会根据照片的exif信息中的拍照时间插入视频。<br>注意exif信息有可能在转换过程中被删除。<br>微信传输图片需要使用原图，否则exif信息将被删除。<br>时间不在轨迹时间中的图片将不会被插入视频。`,
			},
			defaultVal: "strava",
			Val:        []string{"strava", "local", "none"},
			Info:       []string{"从strava取照片", `从<a href="%s">图片管理</a>取照片`, "不增加照片"},
		},
	}
	show_index = append(show_index, "photos_dir")

	photosTimezoneOption = &PhotosTimezoneOption{
		Float64Option: Float64Option{
			BaseOption: BaseOption{
				shortInfo: "照片所在的时区值",
				longInfo:  "因为轨迹文件提供的时间是UTC时间，而exif信息中的拍照时间是当地时间，这就需要有个转换过程。<br>格式举例:8或者-11或者3.5。<br>如果不设置则自动从轨迹信息中取得时区信息。",
			},
		},
	}
	makevideoOptions["photos_timezone"] = photosTimezoneOption
	show_index = append(show_index, "photos_timezone")

	makevideoOptions["photos_show_secs"] = &Int64Option{
		BaseOption: BaseOption{
			shortInfo: "照片显示秒数",
			longInfo:  "不设置则自动被设置为2秒。",
		},
		defaultVal: "2",
		min:        1,
	}
	show_index = append(show_index, "photos_show_secs")

	makevideoOptions["sendemail"] = &SendEmailOption{
		BoolOption: BoolOption{
			BaseOption: BaseOption{
				shortInfo: "发送邮件",
				longInfo:  "视频生成成功后，是否发到Strava注册信箱。",
				required:  true,
			},
		},
		defaultVal: true,
	}
	show_index = append(show_index, "video_width")

	for index, option := range makevideoOptions {
		option.Init(index)
	}
}

type MakeVideoOptions struct {
	TrackId         int64
	UseStravaPhotos bool
	StravaPhotoSize int64
}

func makevideoHandler(w http.ResponseWriter, r *http.Request) {
	uid, token, err := checkCookie(r)
	if err != nil {
		httpCookieError(w)
		return
	}

	status, err := users.GetUserStatus(uid)
	if err != nil {
		log.Println(uid, "makevideoHandler users.GetUserStatus:", err)
		w.WriteHeader(403)
	}
	if status == UserMakingVideo {
		httpReturnHome(w, "正在生成一个视频")
		return
	}

	client := strava.NewClient(token)
	service := strava.NewCurrentAthleteService(client)

	if r.Method == "POST" {
		moptions := new(MakeVideoOptions)

		r.ParseForm()

		output_dir := filepath.Join(users.dir, fmt.Sprintf("%d", uid), "output")
		if dir_check_creat(output_dir, true) != nil {
			log.Println(uid, "makevideoHandler dir_check_creat:", err)
			w.WriteHeader(403)
			return
		}

		var video_width, video_height, video_border int64

		gpx_name := filepath.Join(output_dir, "g2v.gpx")

		config := "[required]\n"
		config += "ffmpeg=" + serverConf.Ffmpeg + "\n"
		config += "google_map_key=" + serverConf.Google_map_key + "\n"
		config += "gps_file=" + gpx_name + "\n"
		config += "google_map_type=satellite\n"
		for index, option := range makevideoOptions {
			if !option.Getrequired() {
				continue
			}
			form, ok := r.Form[index]
			if !ok {
				httpShowError(w, option.GetshortInfo()+"没有设置")
				return
			}
			c, err := option.Form2Config(form, uid)
			if err != nil {
				httpShowError(w, option.GetshortInfo()+err.Error())
				return
			}
			config += c

			if index == "video_width" || index == "video_height" || index == "video_border" || index == "trackid" {
				num, err := option.(*Int64Option).Form2Int64(form)
				if err != nil {
					httpShowError(w, option.GetshortInfo()+err.Error())
					return
				}
				switch index {
				case "video_width":
					video_width = num
				case "video_height":
					video_height = num
				case "video_border":
					video_border = num
				case "trackid":
					TrackId := num
				}
			}

			delete(r.Form, index)
		}

		//Special check for video_width, video_height, video_border
		b_tmp := video_border * 2
		if b_tmp >= video_width || b_tmp >= video_height {
			httpShowError(w, "你把边框宽度设置这么大浏览器会爆炸的")
			return
		}
		if video_width > video_height {
			moptions.StravaPhotoSize = video_width
		} else {
			moptions.StravaPhotoSize = video_height
		}

		gotPhotosTimezoneOption := false
		moptions.UseStravaPhotos = false
		sendemail = false
		config += "[optional]\n"
		for index, form := range r.Form {
			option, ok := makevideoOptions[index]
			if !ok {
				continue
			}
			if !option.FormHaveData(form) {
				continue
			}
			c, err := option.Form2Config(form, uid)
			if err != nil {
				httpShowError(w, option.GetshortInfo()+err.Error())
				return
			}
			config += c

			switch index {
			case "photos_timezone":
				gotPhotosTimezoneOption = true
			case "photos_dir":
				photo, _ := option.(*PhotosOption).Form2String(form)
				if photo == "strava" {
					moptions.UseStravaPhotos = true
				}
			case "sendemail":
				sendemail = option.(*SendEmailOption).Form2Bool(form)
			}
		}
		//Get activity.StartDate and activity.StartDateLocal
		activity, err := strava.NewActivitiesService(client).Get(trackId).IncludeAllEfforts().Do()
		if err != nil {
			httpShowError(w, "strava出错:"+err.Error())
			return
		}
		if !gotPhotosTimezoneOption {
			c, _ := photosTimezoneOption.Float642Config(activity.StartDateLocal.Sub(activity.StartDate).Hours())
			config += c + "\n"
		}

		config += "output_dir=" + output_dir + "\n"

		config_name := filepath.Join(output_dir, "config.ini")
		config_fp, err := os.Create(config_name)
		if err != nil {
			log.Println(uid, "makevideoHandler os.Create:", config_name, err)
			httpShowError(w, "系统出错:"+err.Error())
			return
		}
		_, err = fmt.Fprintln(config_fp, config)
		config_fp.Close()
		if err != nil {
			log.Println(uid, "makevideoHandler fmt.Fprintln:", config_name, err)
			httpShowError(w, "系统出错:"+err.Error())
			return
		}

		//Track
		streams, err := strava.NewActivityStreamsService(client).Get(trackId, []strava.StreamType{strava.StreamTypes.Location,
			strava.StreamTypes.Elevation,
			strava.StreamTypes.Time}).Do()
		if err != nil {
			httpShowError(w, "strava出错:"+err.Error())
			return
		}
		streams_len := len(streams.Time.Data)
		if streams_len != len(streams.Location.Data) || streams_len != len(streams.Elevation.Data) {
			httpShowError(w, "strava提供轨迹数据有错")
			return
		}

		gpx_file := new(gpx.GPX)
		for i := 0; i < streams_len; i++ {
			if len(streams.Location.Data[i]) != 2 {
				httpShowError(w, "strava提供轨迹数据有错")
				return
			}
			gpx_file.AppendPoint(
				&gpx.GPXPoint{
					Point: gpx.Point{
						Latitude:  streams.Location.Data[i][0],
						Longitude: streams.Location.Data[i][1],
						Elevation: *gpx.NewNullableFloat64(streams.Elevation.Data[i]),
					},
					Timestamp: activity.StartDate.Add(time.Duration(streams.Time.Data[i]) * time.Second),
				})
		}
		gpxBytes, err := gpx_file.ToXml(gpx.ToXmlParams{Version: "1.1", Indent: true})
		if err != nil {
			log.Println(uid, "makevideoHandler gpx_file.ToXml:", err)
			httpShowError(w, "系统出错:"+err.Error())
			return
		}

		//Write to gpx_name
		gpx_fp, err := os.Create(gpx_name)
		if err != nil {
			log.Println(uid, "makevideoHandler os.Create:", gpx_name, err)
			httpShowError(w, "系统出错:"+err.Error())
			return
		}
		_, err = gpx_fp.Write(gpxBytes)
		gpx_fp.Close()
		if err != nil {
			log.Println(uid, "makevideoHandler gpx_fp.Write:", gpx_name, err)
			httpShowError(w, "系统出错:"+err.Error())
			return
		}

		err = users.SetUserStatus(uid, UserMakingVideo, moptions, "")
		if err != nil {
			log.Println(uid, "makevideoHandler users.GetUserStatus:", err)
			w.WriteHeader(403)
			return
		}

		httpReturnHome(w, "开始生成")

		go makeVideo(uid, token, moptions)

		return
	}

	httpHead(w)
	show := `带*的为必填项<br><br>`
	show += `<form action="`
	show += serverConf.DomainDir + web_makevideo
	show += `" method="post">`
	for _, index := range show_index {
		option := makevideoOptions[index]
		if !option.show() {
			continue
		}
		if option.Getrequired() {
			show += `*`
		}
		show += option.GetshortInfo() + `<br>`
		if option.GetlongInfo() != "" {
			show += option.GetlongInfo() + `<br>`
		}
		html, err = option.GetHtmlInput(service, index)
		if err != nil {
			httpShowError(w, err.Error())
			return
		}
		show += html
		show += `<br><br>`
	}
	show += `<input type="submit" value="Submit" /> <input type="reset" value="Reset" /></form>`
	fmt.Fprintln(w, show)
	httpTail(w)
}

func makeVideo(uid uint64, token string, options *MakeVideoOptions) {
	status := UserMakeVideoFail
	var reason string
	defer func() {
		err := users.SetUserStatus(uid, status, nil, reason)
		if err != nil {
			log.Println(uid, "makeVideo users.SetUserStatus:", err)
		}

		sendMail(uid, token, status, reason)
	}()

	output_dir := filepath.Join(users.dir, fmt.Sprintf("%d", uid), "output")
	config_dir := filepath.Join(output_dir, "config.ini")

	if options.UseStravaPhotos {
		reason = "从Strava下载照片出错"

		photos_dir := filepath.Join(output_dir, "photos")
		os.RemoveAll(photos_dir)
		err := dir_check_creat(photos_dir, true)
		if err != nil {
			log.Println("makeVideo dir_check_creat:", photos_dir, err)
			return
		}

		photos, err := strava.NewActivitiesService(strava.NewClient(token)).ListPhotos(options.TrackId).Size(uint(options.StravaPhotoSize)).Do()
		if err != nil {
			log.Println("makeVideo ListPhotos:", photos_dir, err)
			return
		}

		config_fp, err := os.OpenFile(config_dir, os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			log.Println("makeVideo os.OpenFile:", photos_dir, err)
			return
		}

		for i := range photos {
			url := photos[i].Urls[fmt.Sprintf("%d", options.StravaPhotoSize)]
			res, err := http.Get(url)
			if err != nil {
				log.Println("makeVideo http.Get:", photos_dir, err)
				return
			}
			photo := fmt.Sprintf("%d.jpg", i)
			f, err := os.Create(filepath.Join(photos_dir, photo))
			if err != nil {
				log.Println("makeVideo os.Create:", photos_dir, err)
				return
			}
			if _, err := io.Copy(f, res.Body); err != nil {
				log.Println("makeVideo io.Copy:", photos_dir, err)
				return
			}
			f.Close()

			_, err = fmt.Fprintf(config_fp, "\n[%s]\ncreated_at=%s\n", photo, photos[i].CreatedAt.Format(stravaphotos_layout))
			if err != nil {
				log.Println("makeVideo fmt.Fprintln", photos_dir, err)
				return
			}
		}

		config_fp.Close()
	}

	reason = "GPS2Video程序执行出错"
	cmd := exec.Command("python", serverConf.GPS2VideoDir, config_dir)
	out, err := cmd.CombinedOutput()
	out_string := string(out)
	if err != nil {
		log.Println("makeVideo", "cmd.CombinedOutput", output_dir, err, out_string)
		return
	}

	if strings.Contains(out_string, "视频生成成功") {
		err := os.Rename(filepath.Join(output_dir, "v.mp4"),
			filepath.Join(output_dir, "..", "v.mp4"))
		if err != nil {
			log.Println("makeVideo", "os.Rename", output_dir, err)
		}
		status = UserNormal
		reason = ""
	} else {
		log.Println("makeVideo", out_string)
	}
}

func sendMail(uid uint64, token string, status int, reason string) {
	if serverConf.SmtpServer == "" {
		return
	}

	//Get email address
}
