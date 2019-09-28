var control = {};
control.add = function(opts) {
	var singleDown = opts.singleDown;
	var singleMove = opts.singleMove;
	var singleUp = opts.singleUp;
	
	var doubleDown = opts.doubleDown;
	var doubleMove = opts.doubleMove;
	var doubleUp = opts.doubleUp;
	
	var wheelRot = opts.wheelRot;
	
	var startElem = opts.startElem;
	var stopElem = opts.stopElem;
	
	
	function grab(e) {
		var box = startElem.getBoundingClientRect();
		singleDown(e, e.clientX-box.left, e.clientY-box.top, true, false) && e.preventDefault();
	}
	function move(e) {
		var box = startElem.getBoundingClientRect();
		singleMove(e, e.clientX-box.left, e.clientY-box.top, true) && e.preventDefault();
	}
	function drop(e) {
		singleUp(e, true, false) && e.preventDefault();
	}
	
	
	var touch_numb = 0;
	var last_touch_x = NaN;
	var last_touch_y = NaN;
	
	function touchStart(e) {
		if (e.targetTouches.length > 2) return; //TODO: get by identifier. Everywhere
		
		var box = startElem.getBoundingClientRect();
		var prevent = false;
		
		var t0 = e.targetTouches[0];
		var t1 = e.targetTouches[1];
		
		if (e.targetTouches.length == 1) {
			prevent = singleDown(e, t0.clientX-box.left, t0.clientY-box.top, false, false);
		} else {
			if (touch_numb == 0) console.warn("Received touchstart for TWO new touches!");
			prevent = singleUp(e, false, true);
			prevent += doubleDown(e,
				t0.clientX-box.left, t0.clientY-box.top,
				t1.clientX-box.left, t1.clientY-box.top
			);
		}
		
		if (prevent) e.preventDefault();
		touch_numb = e.targetTouches.length;
		last_touch_x = t0.clientX-box.left;
		last_touch_y = t0.clientY-box.top;
	}
	
	function touchMove(e) {
		if (e.targetTouches.length > 2) return;
		if (e.targetTouches.length != touch_numb) return; //тут что-то не так
		
		var box = startElem.getBoundingClientRect();
		
		var t0 = e.targetTouches[0];
		var t1 = e.targetTouches[1];
		
		if (e.targetTouches.length == 1) {
			singleMove(e, t0.clientX-box.left, t0.clientY-box.top, false) && e.preventDefault();
		} else {
			//мобильная Опера 12.04 передёт тачи сюда в обратном порядке
			if (t0.identifier > t1.identifier) {var t=t0; t0=t1; t1=t;}
			doubleMove(e,
				t0.clientX-box.left, t0.clientY-box.top,
				t1.clientX-box.left, t1.clientY-box.top
			) && e.preventDefault();
		}
		
		last_touch_x = t0.clientX-box.left;
		last_touch_y = t0.clientY-box.top;
	}
	
	function touchEnd(e) {
		if (e.targetTouches.length > 1) return;
		
		if (e.targetTouches.length == 0) {
			if (touch_numb == 2) { // если подняли оба пальца сразу
				console.warn("TWO touches ended simultaneously!");
				(doubleUp(e) + singleDown(e, last_touch_x, last_touch_y, false, true)) && e.preventDefault();
			}
			singleUp(e, false, false) && e.preventDefault();
		} else {
			var box = startElem.getBoundingClientRect();
			var t = e.targetTouches[0];
			(doubleUp(e) + singleDown(e, t.clientX-box.left, t.clientY-box.top, false, true)) && e.preventDefault();
		}
		
		touch_numb = e.targetTouches.length;
	}
	
	startElem.addEventListener('mousedown', grab, true);
	startElem.addEventListener('mousemove', move, true);
	stopElem.addEventListener('mouseup', drop, true);
	startElem.addEventListener('touchstart', touchStart, true);
	startElem.addEventListener('touchmove', touchMove, true);
	stopElem.addEventListener('touchend', touchEnd, true);
	
	if (wheelRot) {
		var deltaMode2pixels = [];
		deltaMode2pixels[WheelEvent.DOM_DELTA_PIXEL] = 1;
		deltaMode2pixels[WheelEvent.DOM_DELTA_LINE] = 20;
		deltaMode2pixels[WheelEvent.DOM_DELTA_PAGE] = 50; // а это вообще как?
		
		startElem.addEventListener('wheel', function(e) {
			var box = startElem.getBoundingClientRect();
			var k = deltaMode2pixels[e.deltaMode];
			wheelRot(e,
			         e.deltaX*k, e.deltaY*k, e.deltaZ*k,
			         e.clientX-box.left, e.clientY-box.top) && e.preventDefault();
		}, true)
	}
}
