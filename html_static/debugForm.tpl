<!doctype html>
<html>
<head>

  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>{{.CommandName}} | Bolt - Debug</title>

  <link rel="stylesheet" href="/static/css/foundation.min.css" />
    <link rel="stylesheet" href="/static/css/custom.css" />

    <style>
    textarea {
        height: 140px;
    }
    .row { max-width: 100%; }
    .row {
        /* These are technically the same, but use both */
  overflow-wrap: break-word;
  word-wrap: break-word;
    }
    </style>

  <script src="/static/js/jquery.js"></script>

  <!-- Import the crypto library.  We're using crypto-js -->
  <!-- https://code.google.com/p/crypto-js/ -->
  <script src="/static/js/crypto-js/aes.js"></script>
  <script src=" /static/js/crypto-js/enc-base64-min.js"></script>
  <script src="/static/js/crypto-js/hmac-sha512.js"></script>

  <!-- Import the javascriptbase64 library  -->
  <!-- http://javascriptbase64.googlecode.cFFom -->
  <script src='/static/js/base64.js' type='text/javascript'></script>

  <script type="text/javascript">
    
    //JS function for more / less description for apicalls and command metas
    //initialize map of statuses
    var statusMap = new Map();
    function toggleDescription(longDescription, shortDescription, apicall)
    {
        if (!statusMap.has(apicall)){
            statusMap.set(apicall, "less");
            document.getElementById(apicall+"DescriptionArea").innerHTML = shortDescription;
            if(longDescription.length > 0){
                document.getElementById(apicall+"ToggleButton").innerText = "See More";
                }
            statusMap.set(apicall, "less");
        } else if (statusMap.get(apicall) == "less") {
            document.getElementById(apicall+"DescriptionArea").innerHTML=longDescription;
            document.getElementById(apicall+"ToggleButton").innerText = "See Less";
            statusMap.set(apicall, "more");
        } else if (statusMap.get(apicall) == "more" ) {
            document.getElementById(apicall+"DescriptionArea").innerHTML = shortDescription;
            document.getElementById(apicall+"ToggleButton").innerText = "See More";
            statusMap.set(apicall, "less");
        }
        
    }
         
    function make_base_auth(user, password) {
      var tok = user + ':' + password;
      var hash = btoa(tok);
      return "Basic "+hash;
    }

    $( document ).ready(function() {
      var hash = window.location.hash;
      if (hash!="" && hash!="#") {
          $('[name="builtinaction"]').val(hash.substring(1));
          $('title').text(hash.substring(1));
          $('#apicallforms').hide();
      }

      $( "#makerequest" ).click(function(){
        processAction("request");
      });
      $( "#maketask" ).click(function(){
        processAction("task");
      });
      $( "#makework" ).click(function(){
        processAction("work");
      });
      $( "#makebuiltin" ).click(function(){
        processAction("builtin");
      });

      function processAction(theAction){

        // Get the message body and action URL for the field to be processed
        var msgbody = "";
        var actionURL = "";

        switch (theAction) {
            case "request":
                msgbody = $('[name="requestjson"]').val();
                actionURL = "/request/{{.CommandName}}";
                break;
            case "task":
                msgbody = $('[name="taskjson"]').val();
                actionURL = "/task/{{.CommandName}}";
                break;
            case "work":
                msgbody = $('[name="workjson"]').val();
                actionURL = "/work/{{.CommandName}}";
                break;
            case "builtin":
                msgbody = $('[name="builtinjson"]').val();
                actionURL = $('[name="builtinaction"]').val();
                break;
        }

        // Create a json object with the message to encode and a timestamp
        var payload = {
          "timestamp": Math.floor(Date.now() / 1000).toString(),
          "message": msgbody
        };

        // Create the signed string
        var signature = CryptoJS.HmacSHA512(JSON.stringify(payload), $('[name="hmackey"]').val());

        // Encode the payload and signature to Base64
        var basePayload = Base64.encode(JSON.stringify(payload));
        var baseSignature = Base64.encode(signature.toString());

        // Combine the encoded message with the key signature
        var jsonStr = {
          "data": basePayload,
          "signature": baseSignature
        }

        $.ajax({
          method: 'Post',
          url: actionURL,
          xhrFields: {
            withCredentials: true
          },
          data: JSON.stringify(jsonStr),
          beforeSend: function(xhr) {
            // The username is the hmacgroup name.
            // The password isn't used since the request uses hmac authentication.
            username = $('[name="hmacgroup"]').val();
            password = "password_ignored";
            xhr.setRequestHeader("Authorization", make_base_auth(username, password));
          },
          success: function(data) {
            $("#results").html("<h4>CALL SUCCESS:</h4><pre>"+JSON.stringify(data, null, 4)+"</pre>");
            $('html,body').animate({ scrollTop: $("#results").offset().top });
          },
          error: function(data) {
            $("#results").html("<h4>ENGINE ERROR:</h4><pre>"+JSON.stringify(data, null, 4)+"</pre>");
            $('html,body').animate({ scrollTop: $("#results").offset().top });
          }
        }); // $.ajax

      } // processAction

    }); // $( document ).ready

  </script>
</head>
<body>
<div class="row">
<div class="large-8 columns" style="border-right: 2px solid #ccc; ">

    <div class="row">
      <div class="large-12 columns">
        <h1>Bolt | Request Tool</h1>
        <h2>{{.CommandName}}</h2>

        <!--more / less description button -->
        {{if .CommandInfo}}
            <p style="display:inline" id="{{.CommandName}}DescriptionArea">{{.CommandInfo.ShortDescription}}</p> 
            <a id="{{.CommandName}}ToggleButton" onclick="(toggleDescription({{.CommandInfo.LongDescription}},{{.CommandInfo.ShortDescription}},{{.CommandName}}))" href="javascript:void(0);"></a>
            <script>toggleDescription({{.CommandInfo.LongDescription}},{{.CommandInfo.ShortDescription}},{{.CommandName}});</script>
        {{end}}
      </div>
    </div>
<!--more button goes around here-->

    <div class="row commandinfo">
      <div class="large-12 columns">
           {{if .CommandInfo}}
        <div class="row collapse prefix-radius">
          <div class="small-3 columns">
            <span class="prefix">Required Params</span>
          </div>
          <div class="small-9 columns">
            <input type="text" value="{{.CommandInfo.RequiredParams}}" disabled>
          </div>
        </div>
        <div class="row collapse prefix-radius">
          <div class="small-3 columns">
            <span class="prefix">Result Timeout (ms)</span>
          </div>
          <div class="small-9 columns">
            <input type="text" value="{{.CommandInfo.ResultTimeoutMs}}" disabled>
          </div>
        </div>
        <div class="row collapse prefix-radius">
          <div class="small-3 columns">
            <span class="prefix">Cache Enabled</span>
          </div>
          <div class="small-9 columns">
            <input type="text" value="{{.CommandInfo.Cache.Enabled}}" disabled>
          </div>
        </div>
        <div class="row collapse prefix-radius">
          <div class="small-3 columns">
            <span class="prefix">Commands</span>
          </div>
          <div class="small-9 columns">
            <input type="text" value="{{range .CommandInfo.Commands}}{{.Name}}, {{end}}" disabled>
          </div>
        </div>
        {{end}}

        

        <div class="row collapse prefix-radius">
          <div class="small-3 columns">
            <span class="prefix">HMAC Group Name</span>
          </div>
          <div class="small-9 columns">
            <input type="text" name="hmacgroup" placeholder="HMAC Group Name">
          </div>
        </div>
        <div class="row collapse prefix-radius">
          <div class="small-3 columns">
            <span class="prefix">Group's HMAC Key</span>
          </div>
          <div class="small-9 columns">
            <input type="text" name="hmackey" placeholder="HMAC Key">
          </div>
        </div>
      </div>
    </div>
    <hr>

{{if .CommandInfo}}
    <div class="row" id="apicallforms">
        <div class="large-4 columns">
            <div class="row">
              <div class="large-12 columns">
                <h3>Request</h3>
                <form>
                    <textarea name="requestjson">{{.RequiredParams}}</textarea>
                    <a href="#" class="button right" id="makerequest">Process Request</a>
                </form>
              </div>
            </div>
        </div>
        <div class="large-4 columns">
            <div class="row">
          <div class="large-12 columns">
            <h3>Task</h3>
            <form>
                <textarea name="taskjson">{{.RequiredParams}}</textarea>
                <a href="#" class="button right" id="maketask">Process Task</a>
            </form>
          </div>
      </div></div>
        <div class="large-4 columns">
        <div class="row">
          <div class="large-12 columns">
            <h4>Work</h4>
            <form>
                <textarea name="workjson">{{.RequiredParams}}</textarea>
                <a href="#" class="button right" id="makework">Process Work</a>
            </form>
          </div>
      </div></div>
        <hr>
    </div>
{{end}}

    <div class="row">
      <div class="large-12 columns">
        <h4>Built-In Root Function</h4>
        <div class="row collapse prefix-radius">
          <div class="small-3 columns">
            <span class="prefix">Root Function URL</span>
          </div>
          <div class="small-9 columns">
            <input type="text" name="builtinaction" value="" placeholder="URL path (ie: /pending)" >
          </div>
        </div>
        <form>
            <textarea name="builtinjson">{}</textarea>
            <a href="#" class="button right" id="makebuiltin">Process Root Function</a>
        </form>
      </div>
    </div>
    <hr>

</div>

<div class="large-4 columns" >
      <div id="results"></div>
</div>
</div>

</body>
</html>
