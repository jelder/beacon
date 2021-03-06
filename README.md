# beacon
[![Circle CI](https://circleci.com/gh/jelder/beacon.svg?style=svg)](https://circleci.com/gh/jelder/beacon)

A tiny subset of Google Analytics, in Go.

Beacon provides the familiar 1x1 transparent PNG web tracking image, but on your own servers and domain, and with a simple API. Data is stored in Redis using [HyperLogLog](http://en.wikipedia.org/wiki/HyperLogLog) for uniques. It is very fast, easily handling hundreds of concurrent requests on a free Heroku instance. See the [Blitz.IO report](https://www.blitz.io/report/47babe4602b876cba4fc026ff2758a96).

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.png)](https://heroku.com/deploy)

### API

```javascript
var objectID = "post_" + 1234;
var image = new Image(1,1);
var url = "//beacon.herokuapp.com/" + objectID + ".png";
image.src = url;
```

See the results at https://beacon.herokuapp.com/api/v1/post_1234, which supports CORS.

```json
{
  "visits": 14,
  "uniques": 4
}
```

You can migrate your existing visits and uniques from another platform by POSTing JSON to the Beacon API.

## Demo

![&nbsp](https://beacon.herokuapp.com/beacon_github_repo.png)

There is an invisible image above this line, though GitHub's Markdown [proxies and anonymizes everything](https://help.github.com/articles/why-do-my-images-have-strange-urls/). The real URL is https://beacon.herokuapp.com/beacon_github_repo.png.

See the traffic we've tracked so far here: https://beacon.herokuapp.com/api/v1/beacon_github_repo

## Thanks

This app uses the smallest possible 1x1 transparent PNG, thanks to the awesome work by [Gareth Rees](http://garethrees.org/2007/11/14/pngcrush/).
