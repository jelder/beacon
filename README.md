# beacon
A tiny subset of Google Analytics, in Go.

Beacon provides the familiar 1x1 transparent PNG web tracking image, but on your own servers and with a simple read-only API.

```javascript
var image = new Image(1,1);
url = "//beacon.example.com/beacon.png" + "myTrackingId" 
image.src = url;
}
```

```json
{
  visits: 14,
  uniques: 4
}
```

Data is stored in Redis using HyperLogLog for uniques.