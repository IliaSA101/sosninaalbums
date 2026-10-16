// Package imgfile — один файл фото: заголовок JPEG (ориентация, цветовой профиль, XMP),
// пределы недоверенного файла и декод с учётом памяти. Ничего не знает о папках и
// альбоме: им пользуются library (скан, миниатюры), export/render и album/salbum (проверка размера).
//
// Файлы: jpeghead — один проход по сегментам заголовка (Header, XMPPrefix); exif — ориентация,
// обход IFD (exifWalk) и время съёмки (Head.Taken);
// icc — цветовой профиль и класс Color; limits — CheckSize, CheckDims, OpenRegular, OpenOriented;
// budget — общий бюджет пикселей декода (DecodeBudget).
package imgfile
