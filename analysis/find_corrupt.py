import click
import json
import os
import glob
from tqdm import tqdm

@click.command()
@click.argument('file', type=click.Path(exists=True, resolve_path=True), nargs=-1)
def find_corrupt(file):
    """Takes in a collection of .tagged.json files and looks for common
    indicators of corruption, e.g. bomb time being too high etc.
    
    Example usage:

      $ find_corrupt.py /path/to/tagged/files/dir

      $ find_corrupt.py /path/to/tagged/files/dir/*.tagged.json
    """

    if os.path.isdir(file[0]):
        file = glob.glob(os.path.join(file[0], '*.tagged.json'))

    findings = {}

    for json_file in tqdm(file, desc='Checking files'):
        with open(json_file) as f:
            data = json.load(f)

        # loop through each tick until we find an issue
        for tick in data['ticks']:
            if tick['gameState']['aliveCT'] > 5:
                findings[json_file] = 'Found unexpectedly high aliveCT of {} at tick {}'.format(tick['gameState']['aliveCT'], tick['tick'])
                break

            if tick['gameState']['aliveT'] > 5:
                findings[json_file] = 'Found unexpectedly high aliveT of {} at tick {}'.format(tick['gameState']['aliveT'], tick['tick'])
                break

            if tick['gameState']['meanHealthCT'] > 100:
                findings[json_file] = 'Found unexpectedly high meanHealthCT of {} at tick {}'.format(tick['gameState']['meanHealthCT'], tick['tick'])
                break

            if tick['gameState']['meanHealthT'] > 100:
                findings[json_file] = 'Found unexpectedly high meanHealthT of {} at tick {}'.format(tick['gameState']['meanHealthT'], tick['tick'])
                break

            if tick['gameState']['bombTime'] > 42:
                findings[json_file] = 'Found unexpectedly high bombTime of {} at tick {}'.format(tick['gameState']['bombTime'], tick['tick'])
                break

            if tick['gameState']['roundTime'] < 0:
                findings[json_file] = 'Found unexpectedly low roundTime of {} at tick {}'.format(tick['gameState']['roundTime'], tick['tick'])
                break

            if tick['gameState']['roundTime'] > 160:
                findings[json_file] = 'Found unexpectedly high roundTime of {} at tick {}'.format(tick['gameState']['roundTime'], tick['tick'])
                break

    for f, reason in findings.items():
        print('> Found possibly corrupt file: "{}"'.format(f))
        print('Reason:\n {}\n'.format(reason))

    if len(findings.items()) == 0:
        print('No corrupt files found')

if __name__ == '__main__':
    find_corrupt() # pylint: disable=no-value-for-parameter