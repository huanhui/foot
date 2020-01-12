package proc

import (
	"encoding/json"
	"github.com/PuerkitoBio/goquery"
	"github.com/hu17889/go_spider/core/common/page"
	"github.com/hu17889/go_spider/core/pipeline"
	"github.com/hu17889/go_spider/core/spider"
	"regexp"
	"strconv"
	"tesou.io/platform/foot-parent/foot-api/common/base"
	"tesou.io/platform/foot-parent/foot-core/module/match/service"
	"tesou.io/platform/foot-parent/foot-spider/module/win007/down"
	"tesou.io/platform/foot-parent/foot-spider/module/win007/vo"
	"time"

	"strings"
	"tesou.io/platform/foot-parent/foot-api/module/match/pojo"
	"tesou.io/platform/foot-parent/foot-spider/module/win007"
)

type BaseFaceProcesser struct {
	service.BFScoreService
	service.BFBattleService
	service.BFFutureEventService

	MatchLastList      []*pojo.MatchLast
	Win007idMatchidMap map[string]string
}

func GetBaseFaceProcesser() *BaseFaceProcesser {
	return &BaseFaceProcesser{}
}

func (this *BaseFaceProcesser) Startup() {
	this.Win007idMatchidMap = map[string]string{}

	newSpider := spider.NewSpider(this, "BaseFaceProcesser")

	for _, v := range this.MatchLastList {
		i := v.Ext[win007.MODULE_FLAG]
		bytes, _ := json.Marshal(i)
		matchLastExt := new(pojo.MatchExt)
		json.Unmarshal(bytes, matchLastExt)

		win007_id := matchLastExt.Sid

		this.Win007idMatchidMap[win007_id] = v.Id

		url := strings.Replace(win007.WIN007_BASE_FACE_URL_PATTERN, "${matchId}", win007_id, 1)
		newSpider = newSpider.AddUrl(url, "html")
	}
	newSpider.SetDownloader(down.NewMWin007Downloader())
	newSpider = newSpider.AddPipeline(pipeline.NewPipelineConsole())
	newSpider.SetThreadnum(1).Run()
}

func (this *BaseFaceProcesser) Process(p *page.Page) {
	request := p.GetRequest()
	if !p.IsSucc() {
		base.Log.Info("URL:,", request.Url, p.Errormsg())
		return
	}

	var regex_temp = regexp.MustCompile(`(\d+).htm`)
	win007Id := strings.Split(regex_temp.FindString(request.Url), ".")[0]
	matchId := this.Win007idMatchidMap[win007Id]

	//积分榜
	scoreList := this.score_process(matchId, p)
	//对战历史
	battleList := this.battle_process(matchId, p)
	//未来对战
	futureEventList := this.future_event_process(matchId, p)
	//保存数据
	this.BFScoreService.SaveList(scoreList)
	this.BFBattleService.SaveList(battleList)
	this.BFFutureEventService.SaveList(futureEventList)

}

//处理获取积分榜数据
func (this *BaseFaceProcesser) score_process(matchId string, p *page.Page) []interface{} {
	data_list_slice := make([]interface{}, 0)
	elem_table := p.GetHtmlParser().Find(" table.mytable")
	elem_table.EachWithBreak(func(i int, selection *goquery.Selection) bool {
		//只取前两个table
		if i > 1 {
			return false
		}

		prev := selection.Prev()
		tempTeamId := strings.TrimSpace(prev.Text())

		selection.Find(" tr ").Each(func(i int, selection *goquery.Selection) {
			if i >= 1 {
				val_arr := make([]string, 0)
				selection.Children().Each(func(i int, selection *goquery.Selection) {
					val := selection.Text()
					val_arr = append(val_arr, strings.TrimSpace(val))
				})
				temp := new(pojo.BFScore)
				temp.MatchId = matchId
				temp.TeamId = tempTeamId
				temp.Type = val_arr[0]
				temp.MatchCount, _ = strconv.Atoi(val_arr[1])
				temp.WinCount, _ = strconv.Atoi(val_arr[2])
				temp.DrawCount, _ = strconv.Atoi(val_arr[3])
				temp.FailCount, _ = strconv.Atoi(val_arr[4])
				temp.GetGoal, _ = strconv.Atoi(val_arr[5])
				temp.LossGoal, _ = strconv.Atoi(val_arr[6])
				temp.DiffGoal, _ = strconv.Atoi(val_arr[7])
				temp.Score, _ = strconv.Atoi(val_arr[8])
				temp.Ranking, _ = strconv.Atoi(val_arr[9])
				temp_val := strings.Replace(val_arr[10], "%", "", 1)
				temp.WinRate, _ = strconv.ParseFloat(temp_val, 64)

				data_list_slice = append(data_list_slice, temp)
			}
		})
		return true
	})
	return data_list_slice
}

//处理对战数据获取
func (this *BaseFaceProcesser) battle_process(matchId string, p *page.Page) []interface{} {
	data_list_slice := make([]interface{}, 0)

	var hdata_str string
	p.GetHtmlParser().Find("script").Each(func(i int, selection *goquery.Selection) {
		text := selection.Text()
		if hdata_str == "" && strings.Contains(text, "var vsTeamInfo") {
			hdata_str = text
		} else {
			return
		}
	})
	if hdata_str == "" {
		return data_list_slice
	}

	// 获取script脚本中的，博彩公司信息
	temp_arr := strings.Split(hdata_str, "var vsTeamInfo = ")
	temp_arr = strings.Split(temp_arr[1], ";")
	hdata_str = strings.TrimSpace(temp_arr[0])
	base.Log.Info(hdata_str)

	var hdata_list = make([]*vo.BattleData, 0)
	json.Unmarshal(([]byte)(hdata_str), &hdata_list)

	//入库中
	for _, v := range hdata_list {
		temp := new(pojo.BFBattle)

		temp.MatchId = matchId
		battleMatchDate, _ := time.ParseInLocation("2006-01-02", v.Year+"-"+v.Date, time.Local)
		temp.BattleMatchDate = battleMatchDate
		temp.BattleLeagueId = v.SclassID
		temp.BattleMainTeamId = v.Home
		temp.BattleGuestTeamId = v.Guest

		half_goals := strings.Split(v.HT, "-")
		full_goals := strings.Split(v.FT, "-")
		temp.BattleMainTeamHalfGoals, _ = strconv.Atoi(half_goals[0])
		temp.BattleGuestTeamHalfGoals, _ = strconv.Atoi(half_goals[1])
		temp.BattleMainTeamGoals, _ = strconv.Atoi(full_goals[0])
		temp.BattleGuestTeamGoals, _ = strconv.Atoi(full_goals[1])

		data_list_slice = append(data_list_slice, temp)
	}

	return data_list_slice
}

//处理获取示来对战数据
func (this *BaseFaceProcesser) future_event_process(matchId string, p *page.Page) []interface{} {
	data_list_slice := make([]interface{}, 0)
	elem_table := p.GetHtmlParser().Find(" table.mytable")
	elem_table_len := len(elem_table.Nodes)
	elem_table.Each(func(i int, selection *goquery.Selection) {
		//只取倒数2,3个table
		if i <= (elem_table_len-3) && i != (elem_table_len-1) {
			return
		}

		selection.Find(" tr ").Each(func(i int, selection *goquery.Selection) {
			if i >= 1 {
				val_arr := make([]string, 0)
				selection.Children().Each(func(i int, selection *goquery.Selection) {
					if i == 0 {
						selection.Find("div").Each(func(i int, selection *goquery.Selection) {
							val := selection.Text()
							val_arr = append(val_arr, strings.TrimSpace(val))
						})
					} else {
						val := selection.Text()
						val_arr = append(val_arr, strings.TrimSpace(val))
					}

				})
				temp := new(pojo.BFFutureEvent)
				temp.MatchId = matchId
				temp.EventMatchDate, _ = time.ParseInLocation("2006-01-02", val_arr[0], time.Local)
				temp.EventLeagueId = val_arr[1]
				temp.EventMainTeamId = val_arr[2]
				temp.EventGuestTeamId = val_arr[3]
				temp_val := strings.Replace(val_arr[4], "天", "", 1)
				temp.IntervalDay, _ = strconv.Atoi(temp_val)

				data_list_slice = append(data_list_slice, temp)
			}
		})
	})
	return data_list_slice
}

func (this *BaseFaceProcesser) Finish() {
	base.Log.Info("基本面分析抓取解析完成 \r\n")

}