export function MapURLLayer(map) {
	function lookInURL(map) {
		if (location.hash.length < 3) return
		var t = location.hash.substr(1).split('/')
		var lon = parseFloat(t[0])
		var lat = parseFloat(t[1])
		var level = parseFloat(t[2])
		map.updateLocation(lon, lat, level)
	}

	var update_timeout = -1

	function updateURL(map) {
		update_timeout = -1
		//history.replaceState({}, "", location.pathname+"#"+map.lon+"/"+map.lat+"/"+map.level);
		location.hash = '#' + map.lon + '/' + map.lat + '/' + Math.log(map.zoom) / Math.LN2
	}

	this.onregister = lookInURL.bind(this)
	this.onunregister = function (map) {}

	this.update = function (map) {
		clearTimeout(update_timeout)
		update_timeout = setTimeout(updateURL.bind(this, map), 500)
	}

	this.redraw = function (map) {}
}
