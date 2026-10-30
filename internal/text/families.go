package text

// families упорядочен по группам (script, kids, serif, sans, display), внутри группы — по имени.
// Набор начертаний — 1 (R), 2 (R, B) или 4: иначе faceFile подставил бы обычное
// вместо жирного курсива. Только статические TTF (вариативные sfnt не умеет).
var families = []Family{
	{ID: "badscript", Name: "Bad Script", CSS: "SA Bad Script", Group: "script", Faces: []Face{
		{"BadScript-Regular.ttf", false, false}}},
	{ID: "caveat", Name: "Caveat", CSS: "SA Caveat", Group: "script", Faces: []Face{
		{"Caveat-Regular.ttf", false, false}, {"Caveat-Bold.ttf", true, false}}},
	{ID: "comforter", Name: "Comforter", CSS: "SA Comforter", Group: "script", Faces: []Face{
		{"Comforter-Regular.ttf", false, false}}},
	{ID: "greatvibes", Name: "Great Vibes", CSS: "SA Great Vibes", Group: "script", Faces: []Face{
		{"GreatVibes-Regular.ttf", false, false}}},
	{ID: "lobster", Name: "Lobster", CSS: "SA Lobster", Group: "script", Faces: []Face{
		{"Lobster-Regular.ttf", false, false}}},
	{ID: "marck", Name: "Marck Script", CSS: "SA Marck Script", Group: "script", Faces: []Face{
		{"MarckScript-Regular.ttf", false, false}}},
	{ID: "pacifico", Name: "Pacifico", CSS: "SA Pacifico", Group: "script", Faces: []Face{
		{"Pacifico-Regular.ttf", false, false}}},

	{ID: "amaticsc", Name: "Amatic SC", CSS: "SA Amatic SC", Group: "kids", Faces: []Face{
		{"AmaticSC-Regular.ttf", false, false}, {"AmaticSC-Bold.ttf", true, false}}},
	{ID: "comfortaa", Name: "Comfortaa", CSS: "SA Comfortaa", Group: "kids", Faces: []Face{
		{"Comfortaa-Regular.ttf", false, false}, {"Comfortaa-Bold.ttf", true, false}}},
	{ID: "marmelad", Name: "Marmelad", CSS: "SA Marmelad", Group: "kids", Faces: []Face{
		{"Marmelad-Regular.ttf", false, false}}},
	{ID: "neucha", Name: "Neucha", CSS: "SA Neucha", Group: "kids", Faces: []Face{
		{"Neucha-Regular.ttf", false, false}}},
	{ID: "nunito", Name: "Nunito", CSS: "SA Nunito", Group: "kids", Faces: []Face{
		{"Nunito-Regular.ttf", false, false}, {"Nunito-Bold.ttf", true, false},
		{"Nunito-Italic.ttf", false, true}, {"Nunito-BoldItalic.ttf", true, true}}},
	{ID: "pangolin", Name: "Pangolin", CSS: "SA Pangolin", Group: "kids", Faces: []Face{
		{"Pangolin-Regular.ttf", false, false}}},
	{ID: "rubik", Name: "Rubik", CSS: "SA Rubik", Group: "kids", Faces: []Face{
		{"Rubik-Regular.ttf", false, false}, {"Rubik-Bold.ttf", true, false}}},

	{ID: "alice", Name: "Alice", CSS: "SA Alice", Group: "serif", Faces: []Face{
		{"Alice-Regular.ttf", false, false}}},
	{ID: "cormorant", Name: "Cormorant Garamond", CSS: "SA Cormorant Garamond", Group: "serif", Faces: []Face{
		{"CormorantGaramond-Regular.ttf", false, false}, {"CormorantGaramond-Bold.ttf", true, false},
		{"CormorantGaramond-Italic.ttf", false, true}, {"CormorantGaramond-BoldItalic.ttf", true, true}}},
	{ID: "forum", Name: "Forum", CSS: "SA Forum", Group: "serif", Faces: []Face{
		{"Forum-Regular.ttf", false, false}}},
	{ID: "kurale", Name: "Kurale", CSS: "SA Kurale", Group: "serif", Faces: []Face{
		{"Kurale-Regular.ttf", false, false}}},
	{ID: "lora", Name: "Lora", CSS: "SA Lora", Group: "serif", Faces: []Face{
		{"Lora-Regular.ttf", false, false}, {"Lora-Bold.ttf", true, false},
		{"Lora-Italic.ttf", false, true}, {"Lora-BoldItalic.ttf", true, true}}},
	{ID: "oldstandard", Name: "Old Standard TT", CSS: "SA Old Standard TT", Group: "serif", Faces: []Face{
		{"OldStandard-Regular.ttf", false, false}, {"OldStandard-Bold.ttf", true, false}}},
	{ID: "playfair", Name: "Playfair Display", CSS: "SA Playfair Display", Group: "serif", Faces: []Face{
		{"PlayfairDisplay-Regular.ttf", false, false}, {"PlayfairDisplay-Bold.ttf", true, false},
		{"PlayfairDisplay-Italic.ttf", false, true}, {"PlayfairDisplay-BoldItalic.ttf", true, true}}},
	{ID: "prata", Name: "Prata", CSS: "SA Prata", Group: "serif", Faces: []Face{
		{"Prata-Regular.ttf", false, false}}},
	{ID: "ptserif", Name: "PT Serif", CSS: "SA PT Serif", Group: "serif", Faces: []Face{
		{"PT_Serif-Web-Regular.ttf", false, false}, {"PT_Serif-Web-Bold.ttf", true, false},
		{"PT_Serif-Web-Italic.ttf", false, true}, {"PT_Serif-Web-BoldItalic.ttf", true, true}}},
	{ID: "yeseva", Name: "Yeseva One", CSS: "SA Yeseva One", Group: "serif", Faces: []Face{
		{"YesevaOne-Regular.ttf", false, false}}},

	{ID: "golos", Name: "Golos Text", CSS: "SA Golos Text", Group: "sans", Faces: []Face{
		{"GolosText-Regular.ttf", false, false}, {"GolosText-Bold.ttf", true, false}}},
	{ID: "montserrat", Name: "Montserrat", CSS: "SA Montserrat", Group: "sans", Faces: []Face{
		{"Montserrat-Regular.ttf", false, false}, {"Montserrat-Bold.ttf", true, false},
		{"Montserrat-Italic.ttf", false, true}, {"Montserrat-BoldItalic.ttf", true, true}}},
	{ID: "oswald", Name: "Oswald", CSS: "SA Oswald", Group: "sans", Faces: []Face{
		{"Oswald-Regular.ttf", false, false}, {"Oswald-Bold.ttf", true, false}}},
	{ID: "raleway", Name: "Raleway", CSS: "SA Raleway", Group: "sans", Faces: []Face{
		{"Raleway-Regular.ttf", false, false}, {"Raleway-Bold.ttf", true, false},
		{"Raleway-Italic.ttf", false, true}, {"Raleway-BoldItalic.ttf", true, true}}},
	{ID: "robotocondensed", Name: "Roboto Condensed", CSS: "SA Roboto Condensed", Group: "sans", Faces: []Face{
		{"RobotoCondensed-Regular.ttf", false, false}, {"RobotoCondensed-Bold.ttf", true, false}}},
	{ID: "tenorsans", Name: "Tenor Sans", CSS: "SA Tenor Sans", Group: "sans", Faces: []Face{
		{"TenorSans-Regular.ttf", false, false}}},

	{ID: "poiretone", Name: "Poiret One", CSS: "SA Poiret One", Group: "display", Faces: []Face{
		{"PoiretOne-Regular.ttf", false, false}}},
	{ID: "ruslan", Name: "Ruslan Display", CSS: "SA Ruslan Display", Group: "display", Faces: []Face{
		{"RuslanDisplay-Regular.ttf", false, false}}},
	{ID: "underdog", Name: "Underdog", CSS: "SA Underdog", Group: "display", Faces: []Face{
		{"Underdog-Regular.ttf", false, false}}},
}
