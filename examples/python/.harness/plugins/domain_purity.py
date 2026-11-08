import sys

from grimp import ImportGraph
from importlinter import Contract, ContractCheck, fields, output
from importlinter.domain.helpers import module_expressions_to_modules


class DomainPurityContract(Contract):
    source_modules = fields.SetField(subfield=fields.ModuleExpressionField())
    allowed_modules = fields.SetField(subfield=fields.ModuleField(), required=False)

    def check(self, graph: ImportGraph, verbose: bool) -> ContractCheck:
        allowed = [module.name for module in self.allowed_modules or ()]
        domains = sorted(module.name for module in module_expressions_to_modules(graph, self.source_modules))
        violations = []
        for domain in domains:
            for importer in sorted({domain} | graph.find_descendants(domain)):
                for imported in sorted(graph.find_modules_directly_imported_by(importer)):
                    if _inside(imported, [domain, *allowed]) or imported.split(".")[0] in sys.stdlib_module_names:
                        continue
                    details = graph.get_import_details(importer=importer, imported=imported)
                    lines = sorted({detail["line_number"] for detail in details})
                    violations.append({"domain": domain, "importer": importer, "imported": imported, "lines": lines})
        return ContractCheck(kept=not violations, metadata={"violations": violations})

    def render_broken_contract(self, check: ContractCheck) -> None:
        for violation in check.metadata["violations"]:
            lines = ", ".join(f"l.{line}" for line in violation["lines"])
            output.print_error(f"{violation['domain']} is not allowed to import {violation['imported']}:")
            output.new_line()
            output.print_error(f"-   {violation['importer']} -> {violation['imported']} ({lines})", bold=False)
            output.new_line()


def _inside(module: str, packages: list[str]) -> bool:
    return any(module == package or module.startswith(package + ".") for package in packages)
