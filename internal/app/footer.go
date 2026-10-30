package app

func renderFooter(c *canvas, help string) {
	if c.h == 0 || c.w == 0 {
		return
	}
	labels := []struct {
		text  string
		color rgb
	}{{"new", blue}, {"edited", green}, {"removed", red}}
	width := 2
	for _, label := range labels {
		width += textWidth(label.text)
	}
	x := max(0, c.w-width-1)
	c.text(1, c.h-1, help, gray, max(0, x-2))
	for _, label := range labels {
		c.text(x, c.h-1, label.text, label.color, c.w-x)
		x += textWidth(label.text) + 1
	}
}
