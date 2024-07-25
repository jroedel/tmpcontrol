package tmpcontrol

import "fmt"

type TempType int

const (
	FermentationTemp TempType = iota + 1
	RoomTemp
	FridgeTemp
)

func (t TempType) String() string {
	switch t {
	case FermentationTemp:
		return "temp"
	case RoomTemp:
		return "ext_temp"
	case FridgeTemp:
		return "aux_temp"
	default:
		panic(fmt.Sprintf("Unknown temperature type: %#v", t))
	}
	return ""
}

//const BREWFATHER_URL="http://log.brewfather.net/stream?id=xxxxxxxxxxxxx";
//
//function testSendTempToBrewfather() {
//sendTempToBrewfather('brew-pi', 21.812, 'Fridge');
//}
//
///**
// * @param {string} deviceName
// * @param {number} tempInC
// * @param {string} tempKind
// */
//function sendTempToBrewfather(deviceName, tempInC, tempKind) {
////     "temp": 20.32,
////     "aux_temp": 15.61, // Fridge Temp
////     "ext_temp": 6.51, // Room Temp
//let tempKeys = {
//'Fridge': 'aux_temp',
//'Room' : 'ext_temp',
//'Fermentation': 'temp'
//} ;
//if (! tempKeys.hasOwnProperty(tempKind)) {
//tempKind = 'Fridge';
//}
//let data = {
//"name": deviceName,
//"temp_unit": "C",
//};
//data[tempKeys[tempKind]] = tempInC;
//var options = {
//'method' : 'post',
//'payload' : data
//};
//console.log("Sending "+JSON.stringify(data));
//UrlFetchApp.fetch(BREWFATHER_URL, options);
//}
