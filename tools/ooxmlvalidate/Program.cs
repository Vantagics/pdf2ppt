using DocumentFormat.OpenXml;
using DocumentFormat.OpenXml.Packaging;
using DocumentFormat.OpenXml.Validation;

if (args.Length < 1)
{
    Console.Error.WriteLine("usage: ooxmlvalidate file.pptx [Office2016|Microsoft365]");
    return 2;
}

var version = args.Length > 1 ? Enum.Parse<FileFormatVersions>(args[1]) : FileFormatVersions.Office2016;
using var doc = PresentationDocument.Open(args[0], false);
var errors = new OpenXmlValidator(version).Validate(doc).ToList();
foreach (var e in errors)
{
    Console.WriteLine($"{e.ErrorType}\t{e.Part?.Uri}\t{e.Path?.XPath}\t{e.Description}");
}
Console.WriteLine($"COUNT {errors.Count}");
return errors.Count == 0 ? 0 : 1;
