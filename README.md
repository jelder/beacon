# beacon
A tiny subset of Google Analytics, in Go.

Beacon provides the familiar 1x1 transparent PNG web tracking image, but on your own servers and with a simple read-only API.

```javascript
var image = new Image(1,1);
url = "//beacon.herokuapp.com/beacon.png?id=" + "myTrackingId" 
image.src = url;
```

See the results at https://beacon.herokuapp.com/api/myTrackingId, which supports CORS.
```json
{
  "visits": 14,
  "uniques": 4
}
```

Data is stored in Redis using HyperLogLog for uniques. It is very fast, easily handling hundreds of concurrent requests on a free Heroku instance. See the [Blitz.IO report](https://www.blitz.io/report/7a814fea9048b3a38332eed44bbfe466).

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.png)](https://heroku.com/deploy)

## Demo

![&nbsp](https://beacon.herokuapp.com/beacon.png?id=beacon_github_repo)

There is an invisible image above this line, though GitHub's Markdown may mess with it. See the traffic we've tracked so far here: https://beacon.herokuapp.com/api/beacon_github_repo

## Thanks

This app uses the smallest possible 1x1 transparent PNG, thanks to the awesome work by [Gareth Rees](http://garethrees.org/2007/11/14/pngcrush/).
