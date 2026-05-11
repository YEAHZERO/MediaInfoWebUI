package taskprogress

const Prefix = "[进度]"

const (
	StageBootstrap    = "启动"
	StageSubtitle     = "字幕"
	StagePrepare      = "准备"
	StageCaptureStart = "截图开始"
	StageRender       = "渲染"
	StageCaptureDone  = "截图完成"
	StagePackage      = "整理"
	StageUpload       = "上传"
)

type Kind string

const (
	KindStep    Kind = "step"
	KindPercent Kind = "percent"
)

type Event struct {
	Kind    Kind
	Stage   string
	Current int
	Total   int
	Percent float64
	Detail  string
}