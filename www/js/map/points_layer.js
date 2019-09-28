function PointsLayer() {
	this.locs = null
	this.canvas = null
	this.gl = null
	this.mvMatrix = mat4.create()
	this.mouse = {x:0, y:0, r:12, shown:false, moved_dis:0, switched:false}
}

PointsLayer.fs = `
#ifdef GL_ES
precision highp float;
#endif

void main(void) {
	gl_FragColor = vec4(0, 0.1, 0.5, 1.0);
}`

PointsLayer.vs = `
attribute vec2 aVertexPosition;
uniform mat4 uMVMatrix;
uniform float uPointSize;

void main(void) {
	gl_PointSize = uPointSize;
	gl_Position = uMVMatrix * vec4(aVertexPosition, 0, 1.0);
}`

PointsLayer._makeShader = function(gl, vss, fss) {
	let vs = gl.createShader(gl.VERTEX_SHADER)
	gl.shaderSource(vs, vss)
	gl.compileShader(vs)
	if (!gl.getShaderParameter(vs, gl.COMPILE_STATUS)) {
		let err = new Error("vs error:\n"+gl.getShaderInfoLog(vs)+"\n"+vss)
		gl.deleteShader(vs)
		throw err
	}

	let fs = gl.createShader(gl.FRAGMENT_SHADER)
	gl.shaderSource(fs, fss)
	gl.compileShader(fs)
	if (!gl.getShaderParameter(fs, gl.COMPILE_STATUS)) {
		let err = new Error("fs error:\n"+gl.getShaderInfoLog(fs)+"\n"+fss)
		gl.deleteShader(vs)
		gl.deleteShader(fs)
		throw err
	}

	let program = gl.createProgram()
	gl.attachShader(program, vs)
	gl.attachShader(program, fs)
	gl.linkProgram(program)

	if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
		gl.deleteProgram(program);
		gl.deleteShader(vs);
		gl.deleteShader(fs);
		throw new Error("Could not initialise shader")
	}
	return program
}

PointsLayer.prototype.setLocations = function(locs) {
	this.locs = locs
	for (let i=0; i<this.locs.length; i++) {
		let r=Math.random()*0.005, a=Math.random()*Math.PI*2
		this.locs[i][0] += r*Math.cos(a)*2
		this.locs[i][1] += r*Math.sin(a)
	}
}

PointsLayer.prototype._initShader = function() {
	let gl = this.gl
	this.shaderProg = PointsLayer._makeShader(gl, PointsLayer.vs, PointsLayer.fs)
	this.shaderProg.uMVMatrix = gl.getUniformLocation(this.shaderProg, 'uMVMatrix')
	this.shaderProg.uPointSize = gl.getUniformLocation(this.shaderProg, 'uPointSize')
	this.shaderProg.aVertexPosition = gl.getAttribLocation(this.shaderProg, "aVertexPosition")
	gl.useProgram(this.shaderProg)
}

PointsLayer.prototype._initBuffer = function(posConv) {
	let gl = this.gl

	let posBuf = new Float32Array(this.locs.length * 2)
	for (let i=0; i<this.locs.length; i++) {
		posBuf[i*2+0] = posConv.lon2x(this.locs[i][0], 1)
		posBuf[i*2+1] = posConv.lat2y(this.locs[i][1], 1)
	}

	let glPosBuf = gl.createBuffer()
	gl.bindBuffer(gl.ARRAY_BUFFER, glPosBuf)
	gl.bufferData(gl.ARRAY_BUFFER, posBuf, gl.STATIC_DRAW)

	gl.enableVertexAttribArray(this.shaderProg.aVertexPosition)
	gl.bindBuffer(gl.ARRAY_BUFFER, glPosBuf)
	gl.vertexAttribPointer(this.shaderProg.aVertexPosition, 2, gl.FLOAT, false, 0, 0)
}

PointsLayer.prototype._resizeIfNeed = function(map) {
	if (map.canvas.width == this.canvas.width && map.canvas.height == this.canvas.height) return
	this.canvas.width = map.canvas.width
	this.canvas.height = map.canvas.height
	if (!this.gl) {
		this.gl = this.canvas.getContext('webgl')
		this.gl.clearColor(0.1,0.1,0.1,0.2)
		this.gl.disable(this.gl.DEPTH_TEST)
		this.gl.enable(this.gl.BLEND)
		//blendFuncSeparate
		//this.gl.blendEquation(this.gl.FUNC_REVERSE_SUBTRACT)
		//this.gl.blendEquationSeparate(this.gl.FUNC_REVERSE_SUBTRACT, this.gl.FUNC_ADD)
		this.gl.blendFunc(this.gl.ONE, this.gl.ONE)
		this._initShader()
		this._initBuffer(map.conv)
	}
	this.gl.viewport(0, 0, this.gl.canvas.width, this.gl.canvas.height)
}

PointsLayer.prototype.onregister = function(map) {
	this.canvas = document.createElement('canvas')
	this.canvas.style.position = 'absolute'
	this.canvas.style.width = '100%'
	this.canvas.style.height = '100%'
	map.div.appendChild(this.canvas)
}
PointsLayer.prototype.onunregister = function(map) {
	map.div.removeChild(this.canvas)
	this.canvas = null
}
PointsLayer.prototype.update = function(map) {}

PointsLayer.prototype.redraw = function(map) {
	if (!this.locs) return
	this._resizeIfNeed(map)

	let left   = -(map.x_shift - map.canvas.width /2/devicePixelRatio) / map.zoom
	let right  = -(map.x_shift + map.canvas.width /2/devicePixelRatio) / map.zoom
	let top    = -(map.y_shift - map.canvas.height/2/devicePixelRatio) / map.zoom
	let bottom = -(map.y_shift + map.canvas.height/2/devicePixelRatio) / map.zoom
	mat4.ortho(left, right, top, bottom, -1, 1, this.mvMatrix)

	this.gl.clear(this.gl.COLOR_BUFFER_BIT)
	this.gl.uniformMatrix4fv(this.shaderProg.uMVMatrix, false, this.mvMatrix)
	this.gl.uniform1f(this.shaderProg.uPointSize, 2*devicePixelRatio*Math.log2(map.zoom)/9)
	this.shaderProg.uPointSize
	this.gl.drawArrays(this.gl.POINTS, 0, this.locs.length)

	// map.rc.scale(1/devicePixelRatio, 1/devicePixelRatio)
	// map.rc.drawImage(this.canvas, 0, 0)
	// map.rc.scale(devicePixelRatio, devicePixelRatio)

	if (this.mouse.shown) {
		map.rc.strokeStyle = 'gray'
		map.rc.beginPath()
		map.rc.arc(this.mouse.x, this.mouse.y, this.mouse.r, 0, 2*Math.PI, true)
		map.rc.stroke()

		let [lon, lat] = this._mouse2pos(map)
		let [xr, yr] = this._radiusInDeg(map, this.mouse.r, lon, lat)
		let nodes_count = 0
		for (let i=0; i<this.locs.length; i++) {
			let loc = this.locs[i]
			if ((loc[0]-lon)*(loc[0]-lon)/xr/xr + (loc[1]-lat)*(loc[1]-lat)/yr/yr < 1) {
				nodes_count++
			}
		}
		if (nodes_count > 0) {
			let text = nodes_count
			let text_w = map.rc.measureText(text).width
			let text_h = parseInt(map.rc.font)
			let padding = 2
			map.rc.translate(this.mouse.x, this.mouse.y)
			map.rc.fillStyle = 'gray'
			map.rc.fillRect(-text_w/2-padding, -text_h-this.mouse.r-padding, text_w+padding*2, text_h+padding*2)
			map.rc.fillStyle = 'white'
			map.rc.textBaseline = 'top'
			map.rc.fillText(text, -text_w/2, -text_h-this.mouse.r+1)
			map.rc.translate(-this.mouse.x, -this.mouse.y)
		}
	}
}

function range(a, x, b){ return Math.max(a, Math.min(x, b)) }
PointsLayer.prototype._mouse2pos = function(map) {
	let lon = map.x2lon(this.mouse.x + map.top_left_x_shift)
	let lat = map.y2lat(this.mouse.y + map.top_left_y_shift)
	return [lon, lat]
}
PointsLayer.prototype._lon2key = function(lon) {
	let k = 10
	return Math.floor(range(0, 180+lon, 360)/k)*k
}
PointsLayer.prototype._lat2key = function(lat) {
	let k = 10
	return Math.floor(range(0, 90+lat, 180)/k)*k
}
PointsLayer.prototype._lonlat2key = function(lon_key, lat_key) {
	return (lon_key*1000 + lat_key + '').padStart(6, '0')
}
PointsLayer.prototype._pos2key = function(lon, lat) {
	return this._lonlat2key(this._lon2key(lon), this._lat2key(lat))
}
PointsLayer.prototype._radiusInDeg = function(map, r, lon, lat) {
	let xr = map.x2lon(map.lon2x(lon) + r) - lon
	let yr = map.y2lat(map.lat2y(lat) - r) - lat
	return [xr, yr]
}

PointsLayer.prototype.onSingleDown = function(map, e) {
	map.requestRedraw()
	this.mouse.moved_dis = 0
	this.mouse.switched = e.isSwitching
	this.mouse.x = e.x
	this.mouse.y = e.y
	this.mouse.shown = !e.isSwitching
}

PointsLayer.prototype.onSingleMove = function(map, e) {
	map.requestRedraw()
	let dx = e.x - this.mouse.x, dy = e.y - this.mouse.y
	this.mouse.moved_dis += Math.sqrt(dx*dx + dy*dy)
	this.mouse.x = e.x
	this.mouse.y = e.y
	this.mouse.shown = true
}

PointsLayer.prototype.onSingleUp = function(map, e) {
	map.requestRedraw()
	this.mouse.shown = true
}

PointsLayer.prototype.onDoubleDown = function(map, e) {
	map.requestRedraw()
	this.mouse.shown = false
}
