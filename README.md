# beacon
A tiny subset of Google Analytics, in Go.

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.png)](https://heroku.com/deploy)

Beacon provides the familiar 1x1 transparent PNG web tracking image, but on your own servers and with a simple read-only API.

```javascript
var image = new Image(1,1);
url = "//beacon.example.com/beacon.png?id=" + "myTrackingId" 
image.src = url;
```

`GET http://beacon.example.com/api/myTrackingId`
```json
{
  "visits": 14,
  "uniques": 4
}
```

Data is stored in Redis using HyperLogLog for uniques.

![&nbsp](https://beacon.herokuapp.com/beacon.png?id=beacon_github_repo)
