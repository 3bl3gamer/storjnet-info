function MapControlLayer(map) {
	
	function point_distance(x1, y1, x2, y2) {
		return Math.sqrt((x2-x1)*(x2-x1) + (y2-y1)*(y2-y1));
	}
	
	var mouse_x=NaN, mouse_y=NaN;
	var mouse_down = false;
	var touch_dist;
	var delay_fingers_hint_until = 0
	
	
	function singleDown(e, x, y, is_mouse, is_switching) {
		mouse_down = is_mouse;
		mouse_x = x;
		mouse_y = y;
		map.emit('SingleDown', {x:x, y:y, isMouse:is_mouse, isSwitching:is_switching})
		return mouse_down;
	}
	function singleMove(e, x, y, is_mouse) {
		if (!is_mouse && Date.now() > delay_fingers_hint_until)
			map.emit('ControlHint', {type:'use_two_fingers'});
		if (mouse_down) map.move(x-mouse_x, y-mouse_y);
		mouse_x = x;
		mouse_y = y;
		map.emit('SingleMove', {x:x, y:y, isMouse:is_mouse, isDown:mouse_down})
		return mouse_down;
	}
	function singleUp(e, is_mouse, is_switching) {
		var was_down = mouse_down;
		mouse_down = false;
		if (was_down)
			map.emit('SingleUp', {x:mouse_x, y:mouse_y, isMouse:is_mouse, wasDown:was_down, isSwitching:is_switching})
		return was_down;
	}
	
	function wheelRot(e, deltaX, deltaY, deltaZ, x, y) {
		if (e.ctrlKey) {
			map.doSmoothZoom(x, y, Math.pow(2, -deltaY/250));
			return true;
		} else {
			map.emit('ControlHint', {type:'use_control_to_zoom'})
			return false;
		}
	}
	
	
	function doubleDown(e, x0, y0, x1, y1) {
		mouse_x = (x0+x1)*0.5;
		mouse_y = (y0+y1)*0.5;
		touch_dist = point_distance(x0,y0,x1,y1);
		map.emit('DoubleDown', {})
		return true;
	}
	function doubleMove(e, x0, y0, x1, y1) {
		var cx = (x0+x1)*0.5;
		var cy = (y0+y1)*0.5;
		var cd = point_distance(x0,y0,x1,y1);
		map.doZoom(cx, cy, cd/touch_dist);
		map.move(cx-mouse_x, cy-mouse_y);
		mouse_x = cx;
		mouse_y = cy;
		touch_dist = cd;
		return true;
	}
	function doubleUp(e) {
		mouse_x = mouse_y = NaN;
		delay_fingers_hint_until = Date.now() + 1000;
		return true;
	}
	
	var params = {
		singleDown: singleDown,
		singleMove: singleMove,
		singleUp: singleUp,
		
		doubleDown: doubleDown,
		doubleMove: doubleMove,
		doubleUp: doubleUp,
		
		wheelRot: wheelRot,
		
		startElem: map.div,
		stopElem: window
	};
	
	this.onregister = function(map) {
		control.add(params);
	}
	
	this.onunregister = function(map) {
		//control.del(params); TODO
	}
	
	this.update = function(map) {}
	this.redraw = function(map) {}
}

function MapControlHintLayer(controlText, twoFingersText) {
	this.controlText = controlText
	this.twoFingersText = twoFingersText

	this.elem = document.createElement('div')
	this.elem.style.position = 'absolute'
	this.elem.style.width = '100%'
	this.elem.style.height = '100%'
	this.elem.style.display = 'flex'
	this.elem.style.textAlign = 'center'
	this.elem.style.alignItems = 'center'
	this.elem.style.justifyContent = 'center'
	this.elem.style.backgroundColor = 'gray'
	this.elem.style.transition = 'opacity 0.25s ease'
	this.elem.style.opacity = 0
	this.elem.style.pointerEvents = 'none'
	this.elem.style.fontSize = '200%'
	// this.elem.style.color = 'white'

	this._timeout = null
	this._showHint = function(text) {
		clearTimeout(this._timeout)
		this.elem.textContent = text
		this.elem.style.opacity = 0.5
		this._timeout = setTimeout(this._hideHint, 1000)
	}
	this._hideHint = function() {
		clearTimeout(this._timeout)
		this.elem.style.opacity = 0
	}.bind(this)

	this.onregister = function(map) { map.div.appendChild(this.elem) }
	this.onunregister = function(map) { map.div.removeChild(this.elem) }
	this.update = function(map) {}
	this.redraw = function(map) {}

	this.onControlHint = function(map, e) {
		switch (e.type) {
		case 'use_control_to_zoom':
			this._showHint(this.controlText)
			break;
		case 'use_two_fingers':
			this._showHint(this.twoFingersText)
			break;
		}
	}

	this.onMapMove = this._hideHint
	this.onMapZoom = this._hideHint
}