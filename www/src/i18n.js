const lang = document.documentElement.lang || 'en'

export function L(defaultText, ...texts) {
	for (let i = 0; i < texts.length; i += 2) {
		if (texts[i] == lang) return texts[i + 1]
	}
	return defaultText
}
