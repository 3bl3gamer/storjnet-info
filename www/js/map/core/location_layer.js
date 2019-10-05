function MapLocationLayer() {
	this.last_location = null;
	this.watch_id = null;
}

MapLocationLayer.prototype.radius = 32;

MapLocationLayer.prototype._onlocation = function(map, e) {
	this.last_location = e.coords;
	map.requestRedraw();
}

MapLocationLayer.prototype.onregister = function(map) {
	this.watch_id = navigator.geolocation.watchPosition(this._onlocation.bind(this, map));
};

MapLocationLayer.prototype.onunregister = function(map) {
	navigator.geolocation.clearWatch(this.watch_id);
	this.watch_id = null;
};

MapLocationLayer.prototype.update = function(map) {};

MapLocationLayer.prototype.redraw = function(map) {
	if (!this.last_location) return;
	
	var x = -map.top_left_x_shift + map.lon2x(this.last_location.longitude);
	var y = -map.top_left_y_shift + map.lat2y(this.last_location.latitude);
	var rc = map.rc;
	
	rc.beginPath();
	rc.arc(x, y, this.radius, 0,3.1415927*2, false);
	rc.stroke();
};

