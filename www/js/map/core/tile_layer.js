function MapTileLayer(initialTileHost) {
	var self = this;
	
	var tileHost;
	var level_difference, zoom_difference;
	this.setTileHost = function(th) {
		tileHost = th;
		level_difference = -Math.log(tileHost.tile_w)/Math.LN2;
		zoom_difference = 1/tileHost.tile_w;
	}
	if (initialTileHost) this.setTileHost(initialTileHost);
	
	var scale = 1;
	var draw_x_shift,
	    draw_y_shift,
	    draw_i_from,
	    draw_j_from,
	    draw_i_numb,
	    draw_j_numb;
	this._updateDrawParams = function(map) {
		var level_grid_width = 1<<(map.level+level_difference);
		scale = (map.zoom*zoom_difference) / level_grid_width;
		var block_size = tileHost.tile_w * scale;
		var x_shift = -tileHost.conv.lon2x(map.lon, map.zoom) + map.top_left_x_offset;
		var y_shift = -tileHost.conv.lat2y(map.lat, map.zoom) + map.top_left_y_offset;
		
		if (x_shift < 0) {
			draw_x_shift = x_shift % block_size;
			draw_i_from = (-x_shift/block_size)|0;
		} else {
			draw_x_shift = x_shift;
			draw_i_from = 0;
		}
		if (y_shift < 0) {
			draw_y_shift = y_shift % block_size;
			draw_j_from = (-y_shift/block_size)|0;
		} else {
			draw_y_shift = y_shift;
			draw_j_from = 0;
		}
		
		draw_i_numb = Math.min(level_grid_width-draw_i_from, ((map.canvas.dp_width -draw_x_shift)/block_size|0)+1);
		draw_j_numb = Math.min(level_grid_width-draw_j_from, ((map.canvas.dp_height-draw_y_shift)/block_size|0)+1);
		//console.log(scale, draw_i_from, draw_j_from, draw_i_numb, draw_j_numb)
	}
	
	this._drawOneTile = function(map, x, y, scale, i, j) {
		if (!tileHost) return;
		var level = map.level+level_difference;
		
		var drawed = tileHost.tryDrawTile(map,this, x, y, scale, i, j, level, true);
		if (!drawed) {
			drawed = tileHost.tryDrawQuarter(map,this, x, y, scale, i%2, j%2, i>>1, j>>1, level-1);
			if (!drawed) {
				tileHost.tryDrawAsQuarter(map,this, x,y,scale, 0,0, i*2  ,j*2  , level+1);
				tileHost.tryDrawAsQuarter(map,this, x,y,scale, 0,1, i*2  ,j*2+1, level+1);
				tileHost.tryDrawAsQuarter(map,this, x,y,scale, 1,0, i*2+1,j*2  , level+1);
				tileHost.tryDrawAsQuarter(map,this, x,y,scale, 1,1, i*2+1,j*2+1, level+1);
			}
		}
	}
	
	
	this.onregister = function() {};
	this.onunregister = function() {
		if (tileHost) tileHost.clearCache();
	};
	
	this.redraw = function(map) {
		if (!tileHost) return;
		map.rc.save();
		
		this._updateDrawParams(map);
		
		for (var i=0; i<draw_i_numb; i++)
			for (var j=0; j<draw_j_numb; j++)
			{
				var dx = draw_x_shift + i * tileHost.tile_w * scale;
				var dy = draw_y_shift + j * tileHost.tile_w * scale;
				this._drawOneTile(map, dx, dy, scale, draw_i_from+i, draw_j_from+j);
			}
		
		map.rc.restore();
	}
	
	this.update = function(map) {
		
	}
}

