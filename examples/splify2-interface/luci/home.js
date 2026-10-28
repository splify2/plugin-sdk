'use strict';
'require view';
'require rpc';

// The host links this directory as view/splify2/p-example-interface/ and loads this view (L.require) while the setting
// `interface` is `example-interface`. Everything the screen needs (a bundle, styles) lies next to it in luci/.
var BASE = 'view/splify2/p-example-interface/';
var callSettings = rpc.declare({ object: 'splify2', method: 'plugin_settings_get', params: [ 'id' ] });

return view.extend({
	load: function() { return callSettings('example-interface'); },
	render: function(res) {
		var greeting = ((res && res.values) || {}).greeting || 'Hello';
		return E('div', { 'data-base': L.resource(BASE) }, E('h2', {}, greeting));
	},
	handleSave: null, handleSaveApply: null, handleReset: null
});
