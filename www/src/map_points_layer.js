import { mat4 } from 'gl-matrix'
import { onError } from './errors'

/**
 * @param {import('locmap').LocMap} map
 * @param {number} r
 * @param {number} lon
 * @param {number} lat
 * @returns {[number, number]}
 */
function radiusInDeg(map, r, lon, lat) {
	let xr = map.x2lon(map.lon2x(lon) + r) - lon
	let yr = map.y2lat(map.lat2y(lat) - r) - lat
	return [xr, yr]
}

/** @param {HTMLCanvasElement} canvas */
export function mustGetWebGLContext(canvas) {
	const gl = canvas.getContext('webgl')
	if (gl === null) throw new Error('webgl not available')
	return gl
}

/**
 * @template T
 * @param {WebGLRenderingContext} gl
 * @param {T|null} value
 * @returns {T}
 */
export function glMust(gl, value) {
	if (value === null) throw new Error('gl error: ' + gl.getError())
	return value
}

/**
 * @param {WebGLRenderingContext} gl
 * @param {WebGLProgram} program
 * @param {string} name
 * @returns {GLint}
 */
export function mustGetAttributeLocation(gl, program, name) {
	const attr = gl.getAttribLocation(program, name)
	if (attr === -1) throw new Error(`attribute '${name}' not found in program`)
	return attr
}

/**
 * @param {WebGLRenderingContext} gl
 * @param {WebGLProgram} program
 * @param {string} name
 * @returns {WebGLUniformLocation}
 */
export function mustGetUniformLocation(gl, program, name) {
	const uni = gl.getUniformLocation(program, name)
	if (uni === null) throw new Error(`uniform '${name}' not found in program`)
	return uni
}

/**
 * @param {WebGLRenderingContext} gl
 * @param {WebGLShader} shader
 */
export function assertShaderIsOk(gl, shader) {
	const msg = gl.getShaderInfoLog(shader)
	if (msg !== '' && msg !== null) throw new Error(msg)
}

/**
 * @param {WebGLRenderingContext} gl
 * @param {string} vss
 * @param {string} fss
 */
function mustMakeShader(gl, vss, fss) {
	let vs = glMust(gl, gl.createShader(gl.VERTEX_SHADER))
	gl.shaderSource(vs, vss)
	gl.compileShader(vs)
	if (!gl.getShaderParameter(vs, gl.COMPILE_STATUS)) {
		let err = new Error('vs error:\n' + gl.getShaderInfoLog(vs) + '\n' + vss)
		gl.deleteShader(vs)
		throw err
	}

	let fs = glMust(gl, gl.createShader(gl.FRAGMENT_SHADER))
	gl.shaderSource(fs, fss)
	gl.compileShader(fs)
	if (!gl.getShaderParameter(fs, gl.COMPILE_STATUS)) {
		let err = new Error('fs error:\n' + gl.getShaderInfoLog(fs) + '\n' + fss)
		gl.deleteShader(vs)
		gl.deleteShader(fs)
		throw err
	}

	let program = glMust(gl, gl.createProgram())
	gl.attachShader(program, vs)
	gl.attachShader(program, fs)
	gl.linkProgram(program)

	if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
		gl.deleteProgram(program)
		gl.deleteShader(vs)
		gl.deleteShader(fs)
		throw new Error('Could not initialise shader')
	}
	return program
}

const fsSrc = `
void main(void) {
	gl_FragColor = vec4(0, 0.1, 0.5, 1.0);
}`

const vsSrc = `
attribute vec2 aVertexPosition;
uniform mat4 uMVMatrix;
uniform float uPointSize;

void main(void) {
	gl_PointSize = uPointSize;
	gl_Position = uMVMatrix * vec4(aVertexPosition, 0, 1.0);
}`

export function PointsLayer() {
	let map = /**@type {import('locmap').LocMap|null}*/ (null)
	let canvas = /**@type {HTMLCanvasElement|null}*/ (null)
	let gl = /**@type {WebGLRenderingContext|null}*/ (null)
	let locs = /**@type {[number,number][]}*/ ([])
	const mvMatrix = mat4.create()
	const mouse = { x: 0, y: 0, r: 12, shown: false }

	/**@type {WebGLProgram}*/ let shader_prog
	/**@type {WebGLUniformLocation}*/ let shader_uMVMatrix
	/**@type {WebGLUniformLocation}*/ let shader_uPointSize
	/**@type {number}*/ let shader_aVertexPosition

	/** @param {import('locmap').LocMap} map */
	function mouse2pos(map) {
		let lon = map.x2lon(mouse.x + map.getTopLeftXShift())
		let lat = map.y2lat(mouse.y + map.getTopLeftYShift())
		return [lon, lat]
	}

	/** @param {WebGLRenderingContext} gl */
	function mustInitShader(gl) {
		shader_prog = mustMakeShader(gl, vsSrc, fsSrc)
		shader_uMVMatrix = mustGetUniformLocation(gl, shader_prog, 'uMVMatrix')
		shader_uPointSize = mustGetUniformLocation(gl, shader_prog, 'uPointSize')
		shader_aVertexPosition = mustGetAttributeLocation(gl, shader_prog, 'aVertexPosition')
		gl.useProgram(shader_prog)
	}

	/**
	 * @param {WebGLRenderingContext} gl
	 * @param {import('locmap').ProjectionConverter} posConv
	 */
	function mustInitBuffer(gl, posConv) {
		let posBuf = new Float32Array(locs.length * 2)
		for (let i = 0; i < locs.length; i++) {
			posBuf[i * 2 + 0] = posConv.lon2x(locs[i][0], 1)
			posBuf[i * 2 + 1] = posConv.lat2y(locs[i][1], 1)
		}

		let glPosBuf = gl.createBuffer()
		gl.bindBuffer(gl.ARRAY_BUFFER, glPosBuf)
		gl.bufferData(gl.ARRAY_BUFFER, posBuf, gl.STATIC_DRAW)

		gl.enableVertexAttribArray(shader_aVertexPosition)
		gl.bindBuffer(gl.ARRAY_BUFFER, glPosBuf)
		gl.vertexAttribPointer(shader_aVertexPosition, 2, gl.FLOAT, false, 0, 0)
	}

	/** @param {import('locmap').LocMap} map */
	function tryResizeIfNeed(map) {
		if (!canvas) return
		const mapCanvas = map.getCanvas()
		if (gl && mapCanvas.width === canvas.width && mapCanvas.height === canvas.height) return
		canvas.width = mapCanvas.width
		canvas.height = mapCanvas.height
		if (!gl) {
			try {
				gl = mustGetWebGLContext(canvas)
				gl.clearColor(0.1, 0.1, 0.1, 0.2)
				gl.disable(gl.DEPTH_TEST)
				gl.enable(gl.BLEND)
				//blendFuncSeparate
				//this.gl.blendEquation(this.gl.FUNC_REVERSE_SUBTRACT)
				//this.gl.blendEquationSeparate(this.gl.FUNC_REVERSE_SUBTRACT, this.gl.FUNC_ADD)
				gl.blendFunc(gl.ONE, gl.ONE)
				mustInitShader(gl)
				mustInitBuffer(gl, map.getProjConv())
			} catch (ex) {
				onError(ex)
				gl = null
			}
		}
		if (gl) gl.viewport(0, 0, gl.canvas.width, gl.canvas.height)
	}

	/** @param {[number,number][]} locs_ */
	this.setLocations = locs_ => {
		locs = locs_
		for (let i = 0; i < locs.length; i++) {
			let r = Math.random() * 0.005,
				a = Math.random() * Math.PI * 2
			locs[i][0] += r * Math.cos(a) * 2
			locs[i][1] += r * Math.sin(a)
		}
		if (map) map.requestRedraw()
	}

	/** @param {import('locmap').LocMap} map_ */
	this.register = map_ => {
		map = map_
		canvas = document.createElement('canvas')
		canvas.style.position = 'absolute'
		canvas.style.width = '100%'
		canvas.style.height = '100%'
		canvas.style.pointerEvents = 'none'
		map.getWrap().appendChild(canvas)
	}
	/** @param {import('locmap').LocMap} map_ */
	this.unregister = map_ => {
		if (canvas) map_.getWrap().removeChild(canvas)
		map = null
		gl = null
		canvas = null
	}

	/** @param {import('locmap').LocMap} map */
	this.redraw = map => {
		if (locs.length === 0) return
		tryResizeIfNeed(map)

		const mapCanvas = map.getCanvas()
		const mapZoom = map.getZoom()
		let left = (map.getXShift() - mapCanvas.width / 2 / devicePixelRatio) / mapZoom
		let right = (map.getXShift() + mapCanvas.width / 2 / devicePixelRatio) / mapZoom
		let top = (map.getYShift() + mapCanvas.height / 2 / devicePixelRatio) / mapZoom
		let bottom = (map.getYShift() - mapCanvas.height / 2 / devicePixelRatio) / mapZoom
		mat4.ortho(mvMatrix, left, right, top, bottom, -1, 1)

		if (gl) {
			gl.clear(gl.COLOR_BUFFER_BIT)
			gl.uniformMatrix4fv(shader_uMVMatrix, false, mvMatrix)
			gl.uniform1f(shader_uPointSize, (2 * devicePixelRatio * Math.log2(mapZoom)) / 9)
			gl.drawArrays(gl.POINTS, 0, locs.length)
		}

		const rc = map.get2dContext()
		if (mouse.shown && rc) {
			rc.strokeStyle = 'gray'
			rc.beginPath()
			rc.arc(mouse.x, mouse.y, mouse.r, 0, 2 * Math.PI, true)
			rc.stroke()

			let [lon, lat] = mouse2pos(map)
			let [xr, yr] = radiusInDeg(map, mouse.r, lon, lat)
			let nodesCount = 0
			for (let i = 0; i < locs.length; i++) {
				const loc = locs[i]
				const dx = (loc[0] - lon) / xr
				const dy = (loc[1] - lat) / yr
				if (dx * dx + dy * dy < 1) {
					nodesCount++
				}
			}
			if (nodesCount > 0) {
				let text = nodesCount + ''
				let text_w = rc.measureText(text).width
				let text_h = parseInt(rc.font)
				let padding = 2
				rc.translate(mouse.x, mouse.y)
				rc.fillStyle = 'gray'
				rc.fillRect(
					-text_w / 2 - padding,
					-text_h - mouse.r - padding,
					text_w + padding * 2,
					text_h + padding * 2,
				)
				rc.fillStyle = 'white'
				rc.textBaseline = 'top'
				rc.fillText(text, -text_w / 2, -text_h - mouse.r + 1)
				rc.translate(-mouse.x, -mouse.y)
			}
		}
	}

	/** @type {import('locmap').MapEventHandlers} */
	this.onEvent = {
		singleDown(map, e) {
			map.requestRedraw()
			mouse.x = e.x
			mouse.y = e.y
			mouse.shown = !e.isSwitching
		},
		singleMove(map, e) {
			map.requestRedraw()
			mouse.x = e.x
			mouse.y = e.y
			mouse.shown = true
		},
		singleUp(map, e) {
			map.requestRedraw()
			mouse.shown = !e.isSwitching
		},
		doubleDown(map, e) {
			map.requestRedraw()
			mouse.shown = false
		},
		singleHover(map, e) {
			map.requestRedraw()
			mouse.x = e.x
			mouse.y = e.y
			mouse.shown = true
		},
	}
}
