package subtitle

type OutputFormat interface {
	Extension() string
	CodecArgs() []string
}

type PNGFormat struct{}

func (f *PNGFormat) Extension() string {
	return ".png"
}

func (f *PNGFormat) CodecArgs() []string {
	return []string{
		"-vcodec", "png",
		"-compression_level", "3",
	}
}

type JPGFormat struct{}

func (f *JPGFormat) Extension() string {
	return ".jpg"
}

func (f *JPGFormat) CodecArgs() []string {
	return []string{
		"-vcodec", "mjpeg",
		"-qscale:v", "3",
	}
}

func GetOutputFormat(variant string) OutputFormat {
	switch variant {
	case "jpg":
		return &JPGFormat{}
	default:
		return &PNGFormat{}
	}
}