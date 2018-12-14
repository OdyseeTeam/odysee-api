function ready(fn) {
  if (document.attachEvent ? document.readyState === "complete" : document.readyState !== "loading"){
    fn();
  } else {
    document.addEventListener('DOMContentLoaded', fn);
  }
}

function get(url, cb) {
  const request = new XMLHttpRequest();
  request.open('GET', url, true);

  request.onload = function() {
      cb(JSON.parse(request.responseText));
  };

  request.onerror = function() {
    console.error(request);
  };

  request.send();
}

ready(() => {
  get("/api", (data) => {
    document.querySelector("#pre").innerHTML = JSON.stringify(data);
  })
})