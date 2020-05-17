/*function tile2lon(x,z) { return (x/Math.pow(2,z)*360-180); }
function tile2lat(y,z) { var n=Math.PI-2*Math.PI*y/Math.pow(2,z);
	return (180/Math.PI*Math.atan(0.5*(Math.exp(n)-Math.exp(-n)))); }

function lon2tile(lon,zoom) { return (Math.round((lon+180)/360*Math.pow(2,zoom))); }
function lat2tile(lat,zoom) { return (Math.round((1-Math.log(Math.tan(lat*Math.PI/180) +
	1/Math.cos(lat*Math.PI/180))/Math.PI)/2 *Math.pow(2,zoom))); }

function getScale(lat,zoom) { return 156543.034 * Math.abs(Math.cos(lat)) / Math.pow(2,zoom); }*/

export function TileMap(mapDiv, conv) {
	var map = this

	var prev_width = mapDiv.offsetWidth
	var prev_height = mapDiv.offsetHeight

	var lon = 0 //45.02268769309893
	var lat = 0 //53.18542068944694
	var level = 8 //12
	var x_shift = 0
	var y_shift = 0
	var zoom = Math.pow(2, level)
	var min_zoom = 256
	// prettier-ignore
	Object.defineProperties(map, {
		'lon': { get: function(){ return lon } },
		'lat': { get: function(){ return lat } },
		'level': { get: function(){ return level } },
		'x_shift': { get: function(){ return x_shift } },
		'y_shift': { get: function(){ return y_shift } },
		'conv': { get: function(){ return conv } },
		'zoom': { get: function(){ return zoom } },
		'top_left_x_offset': { get: function(){ return prev_width /2 } },
		'top_left_y_offset': { get: function(){ return prev_height/2 } },
		'top_left_x_shift': { get: function(){ return x_shift-prev_width /2 } },
		'top_left_y_shift': { get: function(){ return y_shift-prev_height/2 } },
	});

	var canvas = document.createElement('canvas')
	canvas.style.position = 'absolute'
	canvas.style.left = 0
	canvas.style.top = 0
	canvas.style.width = '100%'
	canvas.style.height = '100%'
	canvas.style.transform = 'translateZ(0)'
	mapDiv.appendChild(canvas)
	var rc = canvas.getContext('2d')
	// prettier-ignore
	Object.defineProperties(map, {
		'div': {get: function(){ return mapDiv } },
		'canvas': { get: function(){ return canvas } },
		'rc': { get: function(){ return rc } }
	});

	this._pos_screen2map = function () {
		lon = conv.x2lon(x_shift, zoom)
		lat = conv.y2lat(y_shift, zoom)
	}

	this._pos_map2screen = function () {
		x_shift = conv.lon2x(lon, zoom)
		y_shift = conv.lat2y(lat, zoom)
	}

	this.lon2x = function (lon) {
		return conv.lon2x(lon, zoom)
	}
	this.lat2y = function (lat) {
		return conv.lat2y(lat, zoom)
	}
	this.lat2scale = function (lat) {
		return conv.getScale(lat, zoom)
	}
	this.x2lon = function (x) {
		return conv.x2lon(x, zoom)
	}
	this.y2lat = function (y) {
		return conv.y2lat(y, zoom)
	}

	//----------
	// core
	//----------

	var layers = []
	this.register = function (layer) {
		var pos = layers.indexOf(layer)
		if (pos != -1) throw new Error('already registered')
		layers.push(layer)
		layer.onregister(this)
	}
	this.unregister = function (layer) {
		var pos = layers.indexOf(layer)
		if (pos == -1) throw new Error('not registered yet')
		layers.splice(pos, 1)
		layer.onunregister(this)
	}

	this.updateLocation = function (_lon, _lat, _level) {
		lon = _lon
		lat = _lat
		level = (_level + 0.5) | 0
		zoom = Math.pow(2, _level)
		this._pos_map2screen()
		this._updateLayers()
		this.requestRedraw()
	}

	this._updateLayers = function () {
		for (var i = 0; i < layers.length; i++) layers[i].update(this)
	}
	this._drawLayers = function (dx, dy, dz) {
		rc.clearRect(0, 0, this.canvas.width, this.canvas.height)
		rc.scale(devicePixelRatio, devicePixelRatio)
		for (var i = 0; i < layers.length; i++) layers[i].redraw(this)
		rc.scale(1 / devicePixelRatio, 1 / devicePixelRatio)
	}

	var zoom_smooth_delta = 1
	var zoom_smooth_x = 0,
		zoom_smooth_y = 0
	this._smoothIfNecessary = function () {
		if (zoom_smooth_delta > 0.99 && zoom_smooth_delta < 1.01) return
		var new_delta = 1 + (zoom_smooth_delta - 1) * 0.7
		this.doZoom(zoom_smooth_x, zoom_smooth_y, zoom_smooth_delta / new_delta)
		zoom_smooth_delta = new_delta
		this.requestRedraw()
	}

	var pending_animation_frame = false
	this.requestRedraw = function () {
		if (!pending_animation_frame) {
			pending_animation_frame = true
			window.requestAnimationFrame(this._onAnimationFrame)
		}
	}
	this._onAnimationFrame = function () {
		pending_animation_frame = false
		map._drawLayers()
		map._smoothIfNecessary()
	}

	//-------------------
	// control inner
	//-------------------
	this.resize = function () {
		var rect = mapDiv.getBoundingClientRect()

		canvas.dp_width = rect.width
		canvas.dp_height = rect.height
		canvas.width = rect.width * devicePixelRatio
		canvas.height = rect.height * devicePixelRatio

		prev_width = rect.width
		prev_height = rect.height

		this.requestRedraw()
	}

	this.doZoom = function (x, y, d) {
		zoom = Math.max(min_zoom, zoom * d)
		level = (Math.log(zoom) / Math.log(2) + 0.5) | 0
		x_shift += (-x + prev_width / 2 - x_shift) * (1 - d)
		y_shift += (-y + prev_height / 2 - y_shift) * (1 - d)
		this._pos_screen2map()

		this._updateLayers()
		this.requestRedraw()
		this.emit('MapZoom', {})
	}

	this.doSmoothZoom = function (x, y, d) {
		zoom_smooth_delta = Math.max(min_zoom / zoom, zoom_smooth_delta * d)
		zoom_smooth_x = x
		zoom_smooth_y = y
		this._smoothIfNecessary()
	}

	this.move = function (dx, dy) {
		x_shift -= dx
		y_shift -= dy
		this._pos_screen2map()

		this._updateLayers()
		this.requestRedraw()
		this.emit('MapMove', {})
	}

	//------------
	// events
	//------------
	this.emit = function (name, params) {
		name = 'on' + name
		for (var i = 0; i < layers.length; i++) {
			var layer = layers[i]
			if (name in layer) layer[name](this, params)
		}
	}

	lon = 0
	lat = 0
	this._pos_map2screen()
}

TileMap.proj = {}

TileMap.proj.Flat = {
	x2lon: function (x, zoom) {
		return x / zoom - 0.5
	},
	y2lat: function (y, zoom) {
		return y / zoom - 0.5
	},

	lon2x: function (lon, zoom) {
		return (lon + 0.5) * zoom
	},
	lat2y: function (lat, zoom) {
		return (lat + 0.5) * zoom
	},
}

TileMap.proj.Mercator = {
	x2lon: function (x, z) {
		return (x / z) * 360 - 180
	},
	y2lat: function (y, z) {
		var n = Math.PI - (2 * Math.PI * y) / z
		return (180 / Math.PI) * Math.atan(0.5 * (Math.exp(n) - Math.exp(-n)))
	},

	lon2x: function (lon, zoom) {
		return ((lon + 180) / 360) * zoom
	},
	lat2y: function (lat, zoom) {
		return (
			((1 - Math.log(Math.tan((lat * Math.PI) / 180) + 1 / Math.cos((lat * Math.PI) / 180)) / Math.PI) /
				2) *
			zoom
		)
	},

	getScale: function (lat, zoom) {
		lat *= Math.PI / 180
		//return 6378245/256 * 2*Math.cos(lat)/(Math.cos(2*lat)+1)/zoom;
		return (156543.034 * 256 * Math.abs(Math.cos(lat))) / zoom
	},
}

TileMap.proj.YandexMercator = {
	//http://www.geofaq.ru/forum/index.php?action=vthread&forum=2&topic=7&page=5#msg1152
	//http://habrahabr.ru/post/151103/
	x2lon: function (x, zoom) {
		return (x / zoom) * 360 - 180
	},
	y2lat: function (y, zoom) {
		var ty = Math.exp((y / zoom) * Math.PI * 2 - Math.PI)
		var m = 5.328478445e-11
		var h = 1.764564338702e-8
		var k = 0.00000657187271079536
		var n = 0.003356551468879694
		var g = Math.PI / 2 - 2 * Math.atan(ty)
		// prettier-ignore
		var l = g + n*Math.sin(2*g) + k*Math.sin(4*g) + h*Math.sin(6*g) + m*Math.sin(8*g);
		return (l * 180) / Math.PI
	},

	lon2x: function (lon, zoom) {
		return ((lon + 180) / 360) * zoom
	},
	lat2y: function (lat, zoom) {
		var l = (lat * Math.PI) / 180
		var k = 0.0818191908426
		var t = k * Math.sin(l)
		// prettier-ignore
		return (
			1 -
			Math.log(
				Math.tan(Math.PI/4 + l/2)
			) / Math.PI +
			k*Math.log(
				Math.tan(
					Math.PI/4 +
					Math.asin(t)/2
				)
			) / Math.PI
		) / 2 * zoom
	},
}
