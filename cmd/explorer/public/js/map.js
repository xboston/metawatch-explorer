var map = L.map('map', {
    center: [20.0, 5.0],
    minZoom: 1,
    zoom: 3,
});

L.tileLayer('http://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
    subdomains: ['a', 'b', 'c']
}).addTo(map);

let markers = L.markerClusterGroup();

for (var i = 0; i < markerPoints.length; ++i) {
    let marker = L.marker(new L.LatLng(markerPoints[i].lat, markerPoints[i].lng));

    let popupContent = 
        '<div style="min-width:250px;min-height:50px"><img width="48" height="48" align="left" src="https://icons.metahash.io/'+markerPoints[i].address+'"/>'
        +'<a href="/address/'+markerPoints[i].address+'/info">'
        +markerPoints[i].name+'</a>'
        +'</br>'
        +markerPoints[i].location
        +'</div>';
    marker.bindPopup(popupContent, {maxWidth: "auto"});
    markers.addLayer(marker);
}

map.addLayer(markers);