package text

// families упорядочен по группам (script, kids, serif, sans), внутри группы — по имени.
// Набор начертаний — 1 (R), 2 (R, B) или 4: иначе faceFile подставил бы обычное
// вместо жирного курсива. Только статические TTF (вариативные sfnt не умеет).
var families = []Family{
	{ID: "badscript", Name: "Bad Script", CSS: "SA Bad Script", Group: "script", Faces: []Face{
		{"BadScript-Regular.ttf", false, false}}},
	{ID: "caveat", Name: "Caveat", CSS: "SA Caveat", Group: "script", Faces: []Face{
		{"Caveat-Regular.ttf", false, false}, {"Caveat-Bold.ttf", true, false}}},
	{ID: "greatvibes", Name: "Great Vibes", CSS: "SA Great Vibes", Group: "script", Faces: []Face{
		{"GreatVibes-Regular.ttf", false, false}}},
	{ID: "marck", Name: "Marck Script", CSS: "SA Marck Script", Group: "script", Faces: []Face{
		{"MarckScript-Regular.ttf", false, false}}},
	{ID: "pacifico", Name: "Pacifico", CSS: "SA Pacifico", Group: "script", Faces: []Face{
		{"Pacifico-Regular.ttf", false, false}}},

	{ID: "amaticsc", Name: "Amatic SC", CSS: "SA Amatic SC", Group: "kids", Faces: []Face{
		{"AmaticSC-Regular.ttf", false, false}, {"AmaticSC-Bold.ttf", true, false}}},
	{ID: "comfortaa", Name: "Comfortaa", CSS: "SA Comfortaa", Group: "kids", Faces: []Face{
		{"Comfortaa-Regular.ttf", false, false}, {"Comfortaa-Bold.ttf", true, false}}},
	{ID: "neucha", Name: "Neucha", CSS: "SA Neucha", Group: "kids", Faces: []Face{
		{"Neucha-Regular.ttf", false, false}}},
	{ID: "nunito", Name: "Nunito", CSS: "SA Nunito", Group: "kids", Faces: []Face{
		{"Nunito-Regular.ttf", false, false}, {"Nunito-Bold.ttf", true, false},
		{"Nunito-Italic.ttf", false, true}, {"Nunito-BoldItalic.ttf", true, true}}},

	{ID: "cormorant", Name: "Cormorant Garamond", CSS: "SA Cormorant Garamond", Group: "serif", Faces: []Face{
		{"CormorantGaramond-Regular.ttf", false, false}, {"CormorantGaramond-Bold.ttf", true, false},
		{"CormorantGaramond-Italic.ttf", false, true}, {"CormorantGaramond-BoldItalic.ttf", true, true}}},
	{ID: "forum", Name: "Forum", CSS: "SA Forum", Group: "serif", Faces: []Face{
		{"Forum-Regular.ttf", false, false}}},
	{ID: "lora", Name: "Lora", CSS: "SA Lora", Group: "serif", Faces: []Face{
		{"Lora-Regular.ttf", false, false}, {"Lora-Bold.ttf", true, false},
		{"Lora-Italic.ttf", false, true}, {"Lora-BoldItalic.ttf", true, true}}},
	{ID: "playfair", Name: "Playfair Display", CSS: "SA Playfair Display", Group: "serif", Faces: []Face{
		{"PlayfairDisplay-Regular.ttf", false, false}, {"PlayfairDisplay-Bold.ttf", true, false},
		{"PlayfairDisplay-Italic.ttf", false, true}, {"PlayfairDisplay-BoldItalic.ttf", true, true}}},
	{ID: "ptserif", Name: "PT Serif", CSS: "SA PT Serif", Group: "serif", Faces: []Face{
		{"PT_Serif-Web-Regular.ttf", false, false}, {"PT_Serif-Web-Bold.ttf", true, false},
		{"PT_Serif-Web-Italic.ttf", false, true}, {"PT_Serif-Web-BoldItalic.ttf", true, true}}},
	{ID: "yeseva", Name: "Yeseva One", CSS: "SA Yeseva One", Group: "serif", Faces: []Face{
		{"YesevaOne-Regular.ttf", false, false}}},

	{ID: "montserrat", Name: "Montserrat", CSS: "SA Montserrat", Group: "sans", Faces: []Face{
		{"Montserrat-Regular.ttf", false, false}, {"Montserrat-Bold.ttf", true, false},
		{"Montserrat-Italic.ttf", false, true}, {"Montserrat-BoldItalic.ttf", true, true}}},
	{ID: "oswald", Name: "Oswald", CSS: "SA Oswald", Group: "sans", Faces: []Face{
		{"Oswald-Regular.ttf", false, false}, {"Oswald-Bold.ttf", true, false}}},
	{ID: "raleway", Name: "Raleway", CSS: "SA Raleway", Group: "sans", Faces: []Face{
		{"Raleway-Regular.ttf", false, false}, {"Raleway-Bold.ttf", true, false},
		{"Raleway-Italic.ttf", false, true}, {"Raleway-BoldItalic.ttf", true, true}}},
}
