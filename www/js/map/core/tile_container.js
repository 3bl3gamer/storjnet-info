function MapTileContainer(tile_w, pathFunc, conv) {
	this.tile_w = tile_w;
	this.pathFunc = pathFunc;
	this.conv = conv;
	this.cache = {};
	
	this.layer_max = 25;
	this.layer_hash_shift = 32;
	this.layer_max_width = 1<<this.layer_max;
}

MapTileContainer.prototype.getHash = function(x,y,z) {
	return (x + y*this.layer_max_width) * this.layer_max + z;
}

MapTileContainer.prototype.getTileImg = function(map, x, y, z) {
	var img = new Image();
	img._loaded = 0;
	img.src = this.pathFunc(x, y, z);
	function onLoad() {
		img._loaded = 1;
		map.requestRedraw();
	}
	img.onload = function() {
		if ('createImageBitmap' in window) {
			// trying no decode image in parallel thread,
			// if failed (beacuse of CORS for example) tryimg to show image anyway
			createImageBitmap(img).then(onLoad, onLoad)
		} else {
			onLoad()
		}
	}
	return img;
}

MapTileContainer.prototype.drawTile = function(map, img, sx,sy, sw,sh, x,y, w,h) {
	var s = devicePixelRatio
	// rounding to real canvas pixels
	var rx = Math.round(x*s)/s
	var ry = Math.round(y*s)/s
	w = Math.round((x+w)*s)/s-rx
	h = Math.round((y+h)*s)/s-ry
	map.rc.drawImage(img, sx,sy, sw,sh, rx,ry, w,h);
}

MapTileContainer.prototype.tryDrawTile = function(map, tileEngine, x,y,scale, i,j,l, load_on_fail) {
	//console.log("drawing tile", x,y,scale, i,j,l)
	var hash = this.getHash(i, j, l);
	var img = this.cache[hash];
	if (img === undefined) {
		if (load_on_fail) {
			this.cache[hash] = this.getTileImg(map, i, j, l);
		}
		return false;
	} else {
		if (img._loaded) {
			var w = this.tile_w;
			this.drawTile(map, img,
			              0,0, w,w,
			              x,y, w*scale,w*scale);
		}
		return img._loaded;
	}
}

MapTileContainer.prototype.tryDrawQuarter = function(map, tileEngine, x,y,scale, qi,qj, i,j,l) {
	var hash = this.getHash(i, j, l);
	var img = this.cache[hash];
	if (!img || !img._loaded) return false;
	var w = this.tile_w/2;
	this.drawTile(map, img,
	              qi*w,qj*w, w,w,
	              x,y, w*2*scale,w*2*scale);
	return true;
}

MapTileContainer.prototype.tryDrawAsQuarter = function(map, tileEngine, x,y,scale, qi,qj, i,j,l) {
	var hash = this.getHash(i, j, l);
	var img = this.cache[hash];
	if (!img || !img._loaded) return false;
	var w = this.tile_w/2*scale;
	this.drawTile(map, img,
	              0,0, this.tile_w,this.tile_w,
	              x+qi*w,y+qj*w, w,w);
	return true;
}

MapTileContainer.prototype.clearCache = function() {
	for (var i in this.cache) delete this.cache[i];
}
