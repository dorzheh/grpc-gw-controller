// Author <dorzheho@cisco.com>

package common

var UrlGenericPart = "http://%s/app/kibana#/discover/87c73750-e022-11e8-8920-5dbbec93f87b?_g=(refreshInterval:(pause:!t,value:0),time:(from:now-7d,mode:quick,to:now))&_a=(columns:!(log),filters:!(%s)),index:'83c5f730-e01e-11e8-8920-5dbbec93f87b',interval:auto,query:(language:lucene,query:''),sort:!('@timestamp',desc))"

var AppNameFilter = "('$state':(store:appState),meta:(alias:!n,disabled:!f,index:'83c5f730-e01e-11e8-8920-5dbbec93f87b',key:kubernetes.annotations.apphc.app.basename,negate:!f,params:(query:%s,type:phrase),type:phrase,value:%s),query:(match:(kubernetes.annotations.apphc.app.basename:(query:%s,type:phrase)))"

var RootGroupIdFilter = "('$state':(store:appState),meta:(alias:!n,disabled:!f,index:'83c5f730-e01e-11e8-8920-5dbbec93f87b',key:kubernetes.annotations.apphc.app.instance.root_group_id,negate:!f,params:(query:%s,type:phrase),type:phrase,value:%s),query:(match:(kubernetes.annotations.apphc.app.instance.root_group_id:(query:%s,type:phrase)))"
