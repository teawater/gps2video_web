package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/strava/go.strava"
	"github.com/tkrajina/gpxgo/gpx"
)

type BaseOption struct {
	//Used by config.ini
	configName string //If not set, will same with index
	required   bool

	//Used by http
	shortInfo  string //If not set, will same with index
	longInfo   string
	inputType  string //If not set, will set as "text"
	defaultVal string

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
	if this.inputType == "" {
		this.inputType = "text"
	}
}

func (this *BaseOption) GetshortInfo() string {
	return this.shortInfo
}

func (this *BaseOption) GetlongInfo() string {
	return this.longInfo
}

func (this *BaseOption) GetinputType() string {
	return this.inputType
}

func (this *BaseOption) GetdefaultVal() string {
	return this.defaultVal
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

type NumberOption struct {
	BaseOption
	min int64
	max int64 //If set to 0, will not check max
}

func (this *NumberOption) Form2Int64(form []string) (num int64, err error) {
	str, err := this.Form2String(form)
	if err != nil {
		return
	}
	num, err = strconv.ParseInt(str, 10, 64)
	return
}

func (this *NumberOption) Form2Config(form []string) (config string, err error) {
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

	config = fmt.Sprintf("%s=%d", this.configName, num)
	return
}

type MakevideoOptioner interface {
	Init(index string)

	GetshortInfo() string
	GetlongInfo() string
	GetinputType() string
	GetdefaultVal() string
	Getrequired() bool

	FormHaveData(form []string) bool
	Form2Config(form []string) (config string, err error)
}

var makevideoOptions map[string]MakevideoOptioner
var show_index []string

func makevideoOptionsInit() {
	makevideoOptions = make(map[string]MakevideoOptioner)
	show_index = make([]string, 0)

	makevideoOptions["video_width"] = &NumberOption{
		BaseOption: BaseOption{
			shortInfo:  "视频宽度",
			longInfo:   "google map免费版的限制，最大640。",
			required:   true,
			defaultVal: "640",
		},
		min: 1,
		max: 640,
	}
	show_index = append(show_index, "video_width")

	makevideoOptions["video_height"] = &NumberOption{
		BaseOption: BaseOption{
			shortInfo:  "视频高度",
			longInfo:   "google map免费版的限制，最大640。",
			required:   true,
			defaultVal: "640",
		},
		min: 1,
		max: 640,
	}
	show_index = append(show_index, "video_height")

	makevideoOptions["video_border"] = &NumberOption{
		BaseOption: BaseOption{
			shortInfo:  "边框宽度",
			longInfo:   "视频中轨迹到边框的距离",
			required:   true,
			defaultVal: "10",
		},
		min: 1,
		max: 640,
	}
	show_index = append(show_index, "video_border")
	makevideoOptions["video_limit_secs"] = &NumberOption{
		BaseOption: BaseOption{
			shortInfo: "生成视频的最大秒数",
			longInfo:  "程序将自动设置选项video_fps, speed, photos_show_secs 和 trackinfo_show_sec。友情提醒：微信朋友圈视频限制时间为10秒。",
		},
		min: 3,
	}
	show_index = append(show_index, "video_limit_secs")

	for index, option := range makevideoOptions {
		option.Init(index)
	}
}

func makevideoHandler(w http.ResponseWriter, r *http.Request) {
	uid, token, err := checkCookie(r)
	if err != nil {
		httpCookieError(w)
		return
	}

	client := strava.NewClient(token)
	output_dir := filepath.Join(users.dir, fmt.Sprintf("%d", uid), "output")

	if r.Method == "POST" {
		r.ParseForm()

		//Get truck
		truck := formGetOne(r, "truck")
		if truck == "" {
			w.WriteHeader(403)
			return
		}
		var truck_id int64
		if truck_id, err = strconv.ParseInt(truck, 10, 64); err != nil {
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
			c, err := option.Form2Config(form)
			if err != nil {
				httpShowError(w, option.GetshortInfo()+err.Error())
				return
			}
			config += c + "\n"

			if index == "video_width" || index == "video_height" || index == "video_border" {
				//err doesn't need check because form is used in option.Form2Config
				num, _ := option.(*NumberOption).Form2Int64(form)
				switch index {
				case "video_width":
					video_width = num
				case "video_height":
					video_height = num
				case "video_border":
					video_border = num
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

		config += "[optional]\n"
		for index, form := range r.Form {
			option, ok := makevideoOptions[index]
			if !ok {
				continue
			}
			if !option.FormHaveData(form) {
				continue
			}
			c, err := option.Form2Config(form)
			if err != nil {
				httpShowError(w, option.GetshortInfo()+err.Error())
				return
			}
			config += c + "\n"
		}
		config += "output_dir=" + output_dir + "\n"

		if dir_check_creat(output_dir, true) != nil {
			log.Println(uid, "makevideoHandler dir_check_creat:", err)
			w.WriteHeader(403)
			return
		}
		need_remove := true
		defer func() {
			if need_remove {
				os.RemoveAll(output_dir)
			}
		}()

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

		//Truck
		//Get activity.StartDate
		activity, err := strava.NewActivitiesService(client).Get(truck_id).IncludeAllEfforts().Do()
		if err != nil {
			httpShowError(w, "strava出错:"+err.Error())
			return
		}

		streams, err := strava.NewActivityStreamsService(client).Get(truck_id, []strava.StreamType{strava.StreamTypes.Location,
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

		os.Remove(filepath.Join(output_dir, "..", "v.mp4"))
		os.Remove(filepath.Join(output_dir, "..", "error"))
		need_remove = false
		httpReturnHome(w, "开始生成")

		go makeVideo(output_dir)

		return
	}

	activities, err := strava.NewCurrentAthleteService(client).ListActivities().Do()
	if err != nil {
		httpShowError(w, "strava出错:"+err.Error())
		return
	}

	exist, err := fileIsExist(output_dir)
	if err != nil {
		log.Println(uid, "makevideoHandler fileIsExist:", output_dir, err)
		w.WriteHeader(403)
		return
	}
	if exist {
		httpReturnHome(w, "正在生成一个视频")
		return
	}

	httpHead(w)
	show := `带*的为必填项<br><br>`
	show += `<form action="`
	show += serverConf.DomainDir + web_makevideo
	show += `" method="post">`
	show += `选择要生成视频的轨迹<br><select name="truck">`
	for _, activity := range activities {
		show += `<option value="` + fmt.Sprintf("%d", activity.Id) + `">`
		show += activity.Name + activity.StartDateLocal.Format(activity_layout)
		show += `</option>`
	}
	show += `</select><br><br>`
	for _, index := range show_index {
		option := makevideoOptions[index]
		if option.Getrequired() {
			show += `*`
		}
		show += option.GetshortInfo() + `<br>`
		show += option.GetlongInfo() + `<br>`
		show += `<input type="` + option.GetinputType() + `" name="` + index + `" value="` + option.GetdefaultVal() + `">`
		show += `<br><br>`
	}
	show += `<input type="submit" value="Submit" /> <input type="reset" value="Reset" /></form>`
	fmt.Fprintln(w, show)
	httpTail(w)
}

func makeVideo(output_dir string) {
	defer os.RemoveAll(output_dir)

	cmd := exec.Command("python", serverConf.GPS2VideoDir, filepath.Join(output_dir, "config.ini"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("makeVideo", "cmd.CombinedOutput", output_dir, err)
		return
	}
	out_string := string(out)

	if strings.Contains(out_string, "视频生成成功") {
		err := os.Rename(filepath.Join(output_dir, "v.mp4"),
			filepath.Join(output_dir, "..", "v.mp4"))
		if err != nil {
			log.Println("makeVideo", "os.Rename", output_dir, err)
		}
	} else {
		log.Println("makeVideo", out_string)
		os.Create(filepath.Join(output_dir, "..", "error"))
	}
}
